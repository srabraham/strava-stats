package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/srabraham/google-oauth-helper/googleauth"
	"github.com/srabraham/strava-oauth-helper/stravaauth"
	"google.golang.org/api/sheets/v4"

	strava "github.com/srabraham/swagger-strava-go"
)

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

func main() {
	flag.Parse()

	scopes := []string{"read_all", "activity:read_all", "profile:read_all"}
	oauthCtx, err := stravaauth.GetOAuth2Ctx(context.Background(), strava.ContextOAuth2, scopes)
	if err != nil {
		log.Fatal(err)
	}

	gClient := getGoogleClient()

	sClient := strava.NewAPIClient(strava.NewConfiguration())
	athlete, _, err := sClient.AthletesApi.GetLoggedInAthlete(oauthCtx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Got athlete:")
	spew.Dump(athlete)

	activities := make([]strava.SummaryActivity, 0)
	for i := 1; ; i++ {
		activitiesPage, _, err := sClient.ActivitiesApi.GetLoggedInAthleteActivities(
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
		log.Printf("Got page %d of activities", i)
		activities = append(activities, activitiesPage...)
	}
	log.Printf("Read %d activities", len(activities))
	if len(activities) > 0 {
		log.Println("Most recent activity:")
		spew.Dump(activities[0])
	}

	sheetsService, err := sheets.New(gClient)
	if err != nil {
		log.Fatal(err)
	}

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
				StringValue: "Activity name",
			}},
		}},
	)
	for _, a := range activities {
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
						StringValue: a.Name,
					},
				},
			},
		})
	}

	rb := &sheets.Spreadsheet{
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
	resp, err := sheetsService.Spreadsheets.Create(rb).Context(context.Background()).Do()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response = %v", resp)
	log.Printf("Spreadsheet is at %s", resp.SpreadsheetUrl)
}
