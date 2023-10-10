package types

import "github.com/srabraham/swagger-strava-go"

type StravaData struct {
	Athlete    swagger.DetailedAthlete   `json:"athlete"`
	Activities []swagger.SummaryActivity `json:"activities"`
}
