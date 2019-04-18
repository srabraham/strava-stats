package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/srabraham/google-oauth-helper/googleauth"
	"github.com/srabraham/strava-oauth-helper/stravaauth"
	"google.golang.org/api/sheets/v4"

	strava "github.com/srabraham/swagger-strava-go"
)

var (
	athleteOutFile    = flag.String("athlete-out-file", "", "File in which to spew athlete details, or blank to not output such a file")
	activitiesOutFile = flag.String("activities-out-file", "", "File in which to spew out all activities, or blank to not output such a file")
	workoutType       = map[int32]string{
		0:  "Run",
		1:  "Foot race",
		2:  "Long run",
		3:  "Run workout",
		10: "Bike",
		11: "Bike race",
		12: "Bike workout",
	}
)

func main() {
	flag.Parse()

	// Do all the auth stuff first
	gClient := getGoogleClient()
	stravaScopes := []string{"read_all", "activity:read_all", "profile:read_all"}
	oauthCtx, err := stravaauth.GetOAuth2Ctx(context.Background(), strava.ContextOAuth2, stravaScopes)
	if err != nil {
		log.Fatal(err)
	}
	sClient := strava.NewAPIClient(strava.NewConfiguration())

	// Fetch things from Strava
	athlete := *getLoggedInAthleteProfile(sClient, &oauthCtx)
	activities := *getLoggedInAthleteActivities(sClient, &oauthCtx)

	if *athleteOutFile != "" {
		err = ioutil.WriteFile(*athleteOutFile, []byte(spew.Sdump(athlete)), 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *activitiesOutFile != "" {
		err = ioutil.WriteFile(*activitiesOutFile, []byte(spew.Sdump(activities)), 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
	// Create a new Spreadsheet and populate it with the Strava data
	sheetsService, err := sheets.New(gClient)
	if err != nil {
		log.Fatal(err)
	}
	ss := createStatsSpreadsheet(&athlete, &activities)
	resp, err := sheetsService.Spreadsheets.Create(ss).Context(context.Background()).Do()
	if err != nil {
		log.Fatal(err)
	}
	// Resize the Sheet to make the columns wide enough.
	_, err = sheetsService.Spreadsheets.BatchUpdate(
		resp.SpreadsheetId,
		&sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
					Dimensions: &sheets.DimensionRange{
						SheetId:   resp.Sheets[0].Properties.SheetId,
						Dimension: "COLUMNS"}}}}}).Do()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response = %v", resp)
	log.Printf("Spreadsheet is at %s", resp.SpreadsheetUrl)
}

func getLoggedInAthleteProfile(sClient *strava.APIClient, oauthCtx *context.Context) *strava.DetailedAthlete {
	athlete, _, err := sClient.AthletesApi.GetLoggedInAthlete(*oauthCtx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Got athlete:")
	spew.Dump(athlete)
	return &athlete
}

func getLoggedInAthleteActivities(sClient *strava.APIClient, oauthCtx *context.Context) *[]strava.SummaryActivity {
	// Fetch all of the logged-in athlete's activities, 200 at a time.
	activities := make([]strava.SummaryActivity, 0)
	for i := 1; ; i++ {
		activitiesPage, _, err := sClient.ActivitiesApi.GetLoggedInAthleteActivities(
			*oauthCtx,
			map[string]interface{}{
				"page":    int32(i),
				"perPage": int32(200),
			},
		)
		if err != nil {
			log.Fatal(err)
		}
		if len(activitiesPage) == 0 {
			// No more Strava activities to fetch.
			break
		}
		log.Printf("Got page %d of activities", i)
		activities = append(activities, activitiesPage...)
	}
	log.Printf("Read %d activities", len(activities))
	if len(activities) > 0 {
		log.Println("Most recent activity:")
		spew.Dump(activities[0])
	}
	return &activities
}

func getGoogleClient() *http.Client {
	if err := googleauth.AddScope("https://www.googleapis.com/auth/spreadsheets"); err != nil {
		log.Fatal(err)
	}
	if err := googleauth.SetTokenFileName("hellostrava-tok"); err != nil {
		log.Fatal(err)
	}
	googleClient, err := googleauth.GetClient()
	if err != nil {
		log.Fatal(err)
	}
	return googleClient
}

func createStatsSpreadsheet(athlete *strava.DetailedAthlete, activities *[]strava.SummaryActivity) *sheets.Spreadsheet {
	rowData := make([]*sheets.RowData, 0)
	rowData = append(rowData, &sheets.RowData{
		Values: []*sheets.CellData{
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Date",
			}},
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Type",
			}},
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Distance (km)",
			}},
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Moving time",
			}},
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Elevation gain (m)",
			}},
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Activity name",
			}},
			{UserEnteredValue: &sheets.ExtendedValue{
				StringValue: "Workout type",
			}},
		}},
	)
	for _, a := range *activities {
		// dur, err := time.ParseDuration(fmt.Sprintf("%ds", a.MovingTime))
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// fmt.Printf("%s, %.2fkm, %v, %s\n", a.StartDateLocal.Format("2006-01-02"), a.Distance/1e3, dur, a.Name)
		rowData = append(rowData, &sheets.RowData{
			Values: []*sheets.CellData{
				{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: a.StartDateLocal.Format("2006-01-02"),
					},
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Pattern: "yyyy-mm-dd",
							Type:    "DATE",
						},
					},
				},
				{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: string(*a.Type_),
					},
				},
				{
					UserEnteredValue: &sheets.ExtendedValue{
						NumberValue: float64(a.Distance) / 1e3,
					},
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Pattern: "0.00",
							Type:    "NUMBER",
						},
					},
				},
				{
					UserEnteredValue: &sheets.ExtendedValue{
						NumberValue: float64(a.MovingTime) / (24 * 60 * 60),
					},
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Pattern: "[h]:mm:ss",
							Type:    "TIME",
						},
					},
				},
				{
					UserEnteredValue: &sheets.ExtendedValue{
						NumberValue: float64(a.TotalElevationGain),
					},
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Pattern: "0.0",
							Type:    "NUMBER",
						},
					},
				},
				{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: a.Name,
					},
				},
				{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: workoutType[a.WorkoutType],
					},
				},
			},
		})
	}

	ss := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: fmt.Sprintf("%s Strava activities for %s %s", time.Now().Format("2006-01-02"), athlete.Firstname, athlete.Lastname),
		},
		Sheets: []*sheets.Sheet{{
			Data: []*sheets.GridData{{
				RowData: rowData,
			}},
			Properties: &sheets.SheetProperties{
				GridProperties: &sheets.GridProperties{
					FrozenRowCount: 1,
				},
			},
		}},
	}
	return ss
}
