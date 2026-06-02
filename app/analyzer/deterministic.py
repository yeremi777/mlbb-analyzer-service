from app.schemas.analysis import AnalyzeResponse, Recommendation
from app.schemas.counter import CounterMatchup


def build_fallback_response(
    target_hero_id: str,
    matchups: list[CounterMatchup],
    limit: int,
) -> AnalyzeResponse:
    recommendations: list[Recommendation] = []

    for rank, matchup in enumerate(matchups[:limit], start=1):
        conditions: list[str] = []
        failure_cases: list[str] = []
        evidence_ids: list[str] = []

        for proof in matchup.proof:
            evidence_ids.append(proof.id)
            conditions.extend(proof.worksBestWhen)
            failure_cases.extend(proof.failureCases)

        recommendations.append(
            Recommendation(
                rank=rank,
                counterHeroId=matchup.counterHeroId,
                score=0,
                confidence=0,
                summary=matchup.reasons[0],
                strengths=matchup.reasons,
                conditions=conditions,
                failureCases=failure_cases,
                evidenceIds=evidence_ids,
            )
        )

    return AnalyzeResponse(
        targetHeroId=target_hero_id,
        source="fallback",
        recommendations=recommendations,
    )

