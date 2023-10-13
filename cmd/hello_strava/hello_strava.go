package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/srabraham/strava-stats/internal/types"

	"github.com/antihax/optional"
	"github.com/davecgh/go-spew/spew"
	"github.com/srabraham/google-oauth-helper/googleauth"
	"github.com/srabraham/strava-oauth-helper/stravaauth"
	strava "github.com/srabraham/swagger-strava-go"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	outputJson  = flag.String("output-json", "", "Optional file for JSON output of Strava data")
	timeout     = flag.Duration("timeout", 30*time.Minute, "an overall timeout on the program")
	workoutType = map[int32]string{
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
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Do all the auth stuff first
	gClient := getGoogleClient()
	stravaScopes := []string{"read_all", "activity:read_all", "profile:read_all"}
	ctx, err := stravaauth.GetOAuth2Ctx(ctx, strava.ContextOAuth2, stravaScopes)
	if err != nil {
		log.Fatal(err)
	}
	sClient := strava.NewAPIClient(strava.NewConfiguration())

	// Fetch things from Strava
	athlete := getLoggedInAthleteProfile(ctx, sClient)
	activities := getLoggedInAthleteActivities(ctx, sClient)

	if *outputJson != "" {
		data := types.StravaData{Activities: activities, Athlete: athlete}
		myJson, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(*outputJson, myJson, 0644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote output JSON to %v", *outputJson)

		// do not submit
		return
	}
	// Create a new Spreadsheet and populate it with the Strava data
	sheetsService, err := sheets.NewService(ctx, option.WithHTTPClient(gClient))
	if err != nil {
		log.Fatal(err)
	}
	ss := createStatsSpreadsheet(&athlete, &activities)
	resp, err := sheetsService.Spreadsheets.Create(ss).Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}
	// Resize the Sheet to make the columns wide enough.
	_, err = sheetsService.Spreadsheets.BatchUpdate(
		resp.SpreadsheetId,
		&sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{
				{
					AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
						Dimensions: &sheets.DimensionRange{
							SheetId:   resp.Sheets[0].Properties.SheetId,
							Dimension: "COLUMNS",
						},
					},
				},
			},
		},
	).Do()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response = %v", resp)
	log.Printf("Spreadsheet is at %s", resp.SpreadsheetUrl)
}

func getLoggedInAthleteProfile(ctx context.Context, sClient *strava.APIClient) strava.DetailedAthlete {
	athlete, _, err := sClient.AthletesApi.GetLoggedInAthlete(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Got athlete:")
	spew.Dump(athlete)
	return athlete
}

func getLoggedInAthleteActivities(ctx context.Context, sClient *strava.APIClient) []strava.SummaryActivity {
	// Fetch all the logged-in athlete's activities, 200 at a time.
	activities := make([]strava.SummaryActivity, 0)
	for i := int32(1); ; i++ {
		activitiesPage, _, err := sClient.ActivitiesApi.GetLoggedInAthleteActivities(
			ctx,
			&strava.ActivitiesApiGetLoggedInAthleteActivitiesOpts{
				Page:    optional.NewInt32(i),
				PerPage: optional.NewInt32(200),
			},
		)
		if err != nil {
			log.Fatal(fmt.Errorf("[GetLoggedInAthleteActivities]: %w", err))
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
	return activities
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

type cellCalc struct {
	header   *sheets.CellData
	cellFunc func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData
}

func header(name string) *sheets.CellData {
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			StringValue: &name,
		},
	}
}

func date(t time.Time) *sheets.CellData {
	format := t.Format("2006-01-02")
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			StringValue: &format,
		},
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{
				Pattern: "yyyy-mm-dd",
				Type:    "DATE",
			},
		},
	}
}

func distanceKm(distM float32) *sheets.CellData {
	f := float64(distM) / 1e3
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			NumberValue: &f,
		},
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{
				Pattern: "0.00",
				Type:    "NUMBER",
			},
		},
	}
}

func distanceMiles(distM float32) *sheets.CellData {
	f := float64(distM) * 0.000621371192
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			NumberValue: &f,
		},
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{
				Pattern: "0.00",
				Type:    "NUMBER",
			},
		},
	}
}

func distanceMeters(distM float32) *sheets.CellData {
	f := float64(distM)
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			NumberValue: &f,
		},
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{
				Pattern: "0.0",
				Type:    "NUMBER",
			},
		},
	}
}

func distanceFeet(distM float32) *sheets.CellData {
	f := float64(distM) * 3.28084
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			NumberValue: &f,
		},
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{
				Pattern: "0",
				Type:    "NUMBER",
			},
		},
	}
}

func duration(sec int32) *sheets.CellData {
	f := float64(sec) / (24 * 60 * 60)
	return &sheets.CellData{
		UserEnteredValue: &sheets.ExtendedValue{
			NumberValue: &f,
		},
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{
				Pattern: "[h]:mm:ss",
				Type:    "TIME",
			},
		},
	}
}

func float64Ptr[N float64 | int32](n N) *float64 {
	f := float64(n)
	return &f
}

func strPtr[S ~string](s S) *string {
	t := string(s)
	return &t
}

func boolPtr[B ~bool](b B) *bool {
	s := bool(b)
	return &s
}

func createStatsSpreadsheet(athlete *strava.DetailedAthlete, activities *[]strava.SummaryActivity) *sheets.Spreadsheet {
	columns := []cellCalc{
		{
			header: header("Date"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return date(activity.StartDateLocal)
			},
		},
		{
			header: header("Type"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return &sheets.CellData{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: strPtr(string(*activity.Type_)),
					},
				}
			},
		},
		{
			header: header("Distance (mi)"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return distanceMiles(activity.Distance)
			},
		},
		{
			header: header("Moving time"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return duration(activity.MovingTime)
			},
		},
		{
			header: header("Elevation gain (ft)"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return distanceFeet(activity.TotalElevationGain)
			},
		},
		{
			header: header("Highest elevation (ft)"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return distanceFeet(activity.ElevHigh)
			},
		},
		{
			header: header("Activity name"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return &sheets.CellData{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: strPtr(activity.Name),
					},
				}
			},
		},
		{
			header: header("Activity type"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return &sheets.CellData{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: strPtr(*activity.Type_),
					},
				}
			},
		},
		//{
		//	header: header("Workout type"),
		//	cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
		//		return &sheets.CellData{
		//			UserEnteredValue: &sheets.ExtendedValue{
		//				StringValue: strPtr(workoutType[activity.WorkoutType]),
		//			},
		//		}
		//	},
		//},
		{
			header: header("URL"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return &sheets.CellData{
					UserEnteredValue: &sheets.ExtendedValue{
						StringValue: strPtr(fmt.Sprintf("https://www.strava.com/activities/%d", activity.Id)),
					},
				}
			},
		},
		{
			header: header("Is Private"),
			cellFunc: func(athlete *strava.DetailedAthlete, activity *strava.SummaryActivity) *sheets.CellData {
				return &sheets.CellData{
					UserEnteredValue: &sheets.ExtendedValue{
						BoolValue: boolPtr(activity.Private),
					},
				}
			},
		},
	}

	rowData := make([]*sheets.RowData, 0)
	theHeader := &sheets.RowData{}
	for _, cc := range columns {
		theHeader.Values = append(theHeader.Values, cc.header)
	}
	rowData = append(rowData, theHeader)

	for _, a := range *activities {
		row := &sheets.RowData{}
		for _, cc := range columns {
			row.Values = append(row.Values, cc.cellFunc(athlete, &a))
		}
		rowData = append(rowData, row)
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
