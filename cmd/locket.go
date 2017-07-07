package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/chug"

	"github.com/spf13/cobra"
	chart "github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/drawing"
)

var (
	acquiredLock = regexp.MustCompile(`lock\..*acquired-lock`)
	releasedLock = regexp.MustCompile(`release\..*released-lock`)
	expiredLock  = regexp.MustCompile(`register-ttl\..*lock-expired`)
)

const (
	NUM_ROWS_PER_SERVER   = 3
	SPACE_BETWEEN_SERVERS = 2
	ACQUIRED_ROW          = 0
	RELEASED_ROW          = 1
	EXPIRED_ROW           = 2
)

func init() {
	RootCmd.AddCommand(locketCommand)
}

type Entries []*Entry

type Entry struct {
	Data    chug.LogEntry
	Errored bool
}

type locketEntries struct {
	acquired Entries
	released Entries
	expired  Entries
}

var locketCommand = &cobra.Command{
	Use:   "locket",
	Short: "Parse and chart locket logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := loadAndParseLogFiles(args)
		if err != nil {
			return err
		}

		writer := &bytes.Buffer{}
		graph := lockReleaseAndExpiredChart(servers)
		err = graph.Render(chart.PNG, writer)

		fmt.Println(writer.String())
		return err
	},
}

func loadAndParseLogFiles(files []string) ([]Entries, error) {
	var servers []Entries

	for _, f := range files {
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		s, err := parse(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}

	return servers, nil
}

func parse(data io.Reader) (Entries, error) {
	entries := Entries{}
	lagerEntries := make(chan chug.Entry)
	go chug.Chug(data, lagerEntries)

	for entry := range lagerEntries {
		if !entry.IsLager {
			continue
		}

		newEntry := &Entry{
			Data:    entry.Log,
			Errored: (entry.Log.LogLevel == lager.ERROR),
		}

		entries = append(entries, newEntry)
	}

	return entries, nil
}

func lockReleaseAndExpiredChart(servers []Entries) chart.Chart {
	var series []chart.Series

	viridisByType := func(xrange, yr chart.Range, index int, x, y float64) drawing.Color {
		// fmt.Println(y)
		color := int(y)
		if y > 3 {
			color = color % (SPACE_BETWEEN_SERVERS * NUM_ROWS_PER_SERVER)
		}
		// fmt.Println(color)
		return chart.Viridis(float64(color)*10, yr.GetMin(), yr.GetMax())
	}

	for i, s := range servers {
		xvalues, yvalues := constructDataPoints(i, s)

		locketSeries := chart.TimeSeries{
			Style: chart.Style{
				Show:             true,
				StrokeWidth:      chart.Disabled,
				DotWidth:         5,
				DotColorProvider: viridisByType,
			},
			XValues: xvalues,
			YValues: yvalues,
		}
		series = append(series, locketSeries)
	}

	return chart.Chart{
		XAxis: chart.XAxis{
			Style: chart.Style{
				Show: true,
			},
			ValueFormatter: chart.TimeMinuteValueFormatter,
		},
		Series: series,
	}
}

func constructDataPoints(serverNum int, data Entries) ([]time.Time, []float64) {
	var xvalues []time.Time
	var yvalues []float64

	for _, e := range data {

		baseYValue := (serverNum * NUM_ROWS_PER_SERVER * SPACE_BETWEEN_SERVERS)
		message := []byte(e.Data.Message)
		if expiredLock.Match(message) {
			xvalues = append(xvalues, e.Data.Timestamp)
			yvalues = append(yvalues, float64(baseYValue+EXPIRED_ROW))
		} else if acquiredLock.Match(message) {
			xvalues = append(xvalues, e.Data.Timestamp)
			yvalues = append(yvalues, float64(baseYValue+ACQUIRED_ROW))
		} else if releasedLock.Match(message) {
			xvalues = append(xvalues, e.Data.Timestamp)
			yvalues = append(yvalues, float64(baseYValue+RELEASED_ROW))
		}
	}

	return xvalues, yvalues
}
