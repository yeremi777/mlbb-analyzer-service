from math import ceil

from fastapi import APIRouter, HTTPException, Query, Request

from app.schemas.analysis import ErrorResponse
from app.schemas.counter import CounterHeroMatchup
from app.schemas.hero import Hero, HeroListResponse

router = APIRouter(prefix="/api/heroes", tags=["heroes"])


@router.get(
    "",
    response_model=HeroListResponse,
    summary="List heroes",
    description=(
        "Returns heroes from the local dataset. Supports frontend filtering by search text, "
        "role, lane, and pagination."
    ),
)
def list_heroes(
    request: Request,
    search: str | None = Query(default=None, description="Case-insensitive search by hero name."),
    role: str | None = Query(default=None, description="Case-insensitive role filter."),
    lane: str | None = Query(default=None, description="Case-insensitive lane filter."),
    page: int = Query(default=1, ge=1, description="1-based page number."),
    size: int = Query(default=10, ge=1, le=100, description="Number of heroes per page."),
) -> HeroListResponse:
    heroes: list[Hero] = request.app.state.dataset.heroes
    filtered = heroes

    if search:
        search_value = search.strip().casefold()
        filtered = [hero for hero in filtered if search_value in hero.name.casefold()]

    if role:
        role_value = role.strip().casefold()
        filtered = [
            hero for hero in filtered if any(hero_role.casefold() == role_value for hero_role in hero.roles)
        ]

    if lane:
        lane_value = lane.strip().casefold()
        filtered = [
            hero for hero in filtered if any(hero_lane.casefold() == lane_value for hero_lane in hero.lanes)
        ]

    total = len(filtered)
    pages = ceil(total / size) if total else 0
    start = (page - 1) * size
    end = start + size

    return HeroListResponse(
        items=filtered[start:end],
        page=page,
        size=size,
        total=total,
        pages=pages,
    )


@router.get(
    "/{hero_id}",
    response_model=Hero,
    summary="Get hero detail",
    description="Returns one hero by dataset UID.",
    responses={404: {"model": ErrorResponse, "description": "Hero was not found."}},
)
def get_hero(hero_id: str, request: Request) -> Hero:
    hero = request.app.state.dataset.heroes_by_id.get(hero_id)
    if hero is None:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "hero_not_found",
                "message": "Hero was not found in the dataset.",
            },
        )
    return hero


@router.get(
    "/{hero_id}/counters",
    response_model=list[CounterHeroMatchup],
    summary="List hero counters",
    description="Returns counter matchup records for one target hero, with counter hero details joined in.",
    responses={404: {"model": ErrorResponse, "description": "Hero or counter data was not found."}},
)
def list_hero_counters(hero_id: str, request: Request) -> list[CounterHeroMatchup]:
    dataset = request.app.state.dataset

    if hero_id not in dataset.heroes_by_id:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "hero_not_found",
                "message": "Hero was not found in the dataset.",
            },
        )

    matchups = dataset.get_matchups_for_target(hero_id)
    if not matchups:
        raise HTTPException(
            status_code=404,
            detail={
                "code": "counter_data_not_found",
                "message": "Counter data was not found for the target hero.",
            },
        )

    return [
        CounterHeroMatchup(
            targetHeroId=matchup.targetHeroId,
            counterHero=dataset.heroes_by_id[matchup.counterHeroId],
            reasons=matchup.reasons,
            counterTypes=matchup.counterTypes,
            proof=matchup.proof,
        )
        for matchup in matchups
    ]
