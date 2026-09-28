package types

import "go.seanabraham.com/swagger-strava-go"

type StravaData struct {
	Athlete    swagger.DetailedAthlete   `json:"athlete"`
	Activities []swagger.SummaryActivity `json:"activities"`
}
