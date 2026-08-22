package domain

import "time"

// HeroRankStat is one hero's current standing at a rank tier and window, as
// served from marts.hero_current.
//
// HeroUID and HeroName are empty when upstream reports a hero the authored
// dataset does not carry yet: absence is naturally representable there, and a
// caller falls back to MainHeroID. The deltas are pointers instead, because a
// zero delta is a real measurement ("did not move") and must not be confused
// with the absence of a prior snapshot.
type HeroRankStat struct {
	MainHeroID      int       `json:"mainHeroId"`
	HeroUID         string    `json:"heroUid,omitempty"`
	HeroName        string    `json:"heroName,omitempty"`
	RankTier        string    `json:"rankTier"`
	WindowDays      int       `json:"windowDays"`
	SnapshotDate    time.Time `json:"snapshotDate"`
	SnapshotAgeDays int       `json:"snapshotAgeDays"`

	WinRate         float64 `json:"winRate"`
	AppearanceShare float64 `json:"appearanceShare"`
	BanRate         float64 `json:"banRate"`

	WinRateDelta         *float64   `json:"winRateDelta"`
	AppearanceShareDelta *float64   `json:"appearanceShareDelta"`
	BanRateDelta         *float64   `json:"banRateDelta"`
	PrevSnapshotDate     *time.Time `json:"prevSnapshotDate"`

	WinRateRank    int `json:"winRateRank"`
	AppearanceRank int `json:"appearanceRank"`

	// Patch context. All three are nil together when the snapshot predates the
	// earliest known patch: an unknown patch is reported, never guessed.
	// WindowCrossesPatch is true when the trailing window reaches back before
	// the patch shipped, so the number cannot be attributed to it alone.
	PatchAsOf          *string `json:"patchAsOf"`
	DaysSincePatch     *int    `json:"daysSincePatch"`
	WindowCrossesPatch *bool   `json:"windowCrossesPatch"`
}

// HeroCounterStat is one observed counter matchup, as served from the counter
// mart. CounterHeroID beats TargetHeroID, and WinRateDelta is how much the
// counter hero's win rate rises when the two meet — an additive delta, always
// positive, never a multiplier.
//
// Source names the upstream list a row came from. Snapshots taken before the
// collector stopped narrowing its request carry only one of the two, so a
// caller comparing days must expect the older ones to be half a matrix.
type HeroCounterStat struct {
	TargetHeroID  int    `json:"targetHeroId"`
	TargetUID     string `json:"targetUid,omitempty"`
	TargetName    string `json:"targetName,omitempty"`
	CounterHeroID int    `json:"counterHeroId"`
	CounterUID    string `json:"counterUid,omitempty"`
	CounterName   string `json:"counterName,omitempty"`

	WinRateDelta float64 `json:"winRateDelta"`
	Source       string  `json:"source"`
	SourceRank   int     `json:"sourceRank"`

	RankTier     string    `json:"rankTier"`
	WindowDays   int       `json:"windowDays"`
	SnapshotDate time.Time `json:"snapshotDate"`
}
