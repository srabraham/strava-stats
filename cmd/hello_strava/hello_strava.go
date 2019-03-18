package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/srabraham/strava-oauth-helper/stravaauth"

	strava "github.com/srabraham/swagger-strava-go"
)

func main() {
	flag.Parse()

	scopes := []string{"read_all", "activity:read_all", "profile:read_all"}
	oauthCtx, err := stravaauth.GetOAuth2Ctx(context.Background(), strava.ContextOAuth2, scopes)
	if err != nil {
		log.Fatal(err)
	}

	client := strava.NewAPIClient(strava.NewConfiguration())
	athlete, _, err := client.AthletesApi.GetLoggedInAthlete(oauthCtx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Got athlete:")
	spew.Dump(athlete)

	activities := make([]strava.SummaryActivity, 0)
	for i := 1; ; i++ {
		activitiesPage, _, err := client.ActivitiesApi.GetLoggedInAthleteActivities(
			oauthCtx,
			map[string]interface{}{
				"page":    int32(i),
				"perPage": int32(200),
			},
		)
		if err != nil {
			log.Fatal(err)
		}
		if len(activitiesPage) == 0 {
			break
		}
		activities = append(activities, activitiesPage...)
	}
	for _, a := range activities {
		dur, err := time.ParseDuration(fmt.Sprintf("%ds", a.MovingTime))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %s: %.2fkm: %v\n", a.Name, a.StartDateLocal.Format("2016-01-02"), a.Distance/1e3, dur)
	}
}
