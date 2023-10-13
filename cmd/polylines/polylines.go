package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/srabraham/strava-stats/internal/types"
	"github.com/twpayne/go-polyline"
)

var (
	inputJson = flag.String("input-json", "", "Input file with Strava data")
)

func main() {
	flag.Parse()
	b, err := os.ReadFile(*inputJson)
	if err != nil {
		log.Fatal(err)
	}
	var sd types.StravaData
	if err = json.Unmarshal(b, &sd); err != nil {
		log.Fatal(err)
	}
	for _, a := range sd.Activities {
		coords, buf, err := polyline.DecodeCoords([]byte(a.Map_.SummaryPolyline))
		if err != nil {
			log.Fatal(fmt.Errorf("[DecodeCoords]: %w", err))
		}
		log.Printf("for activity %v, coords = %v, buf = %v", a.Name, coords, buf)
	}
}
