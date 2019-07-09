package tao

import (
	"expvar"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/xu215740578/logger"
)

var (
	handleExported *expvar.Int
	connExported   *expvar.Int
	timeExported   *expvar.Float
	qpsExported    *expvar.Float
)

func init() {
	handleExported = expvar.NewInt("TotalHandle")
	connExported = expvar.NewInt("TotalConn")
	timeExported = expvar.NewFloat("TotalTime")
	qpsExported = expvar.NewFloat("QPS")
}

func monitorHander(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintf("handleExported %d, connExported %d, timeExported %f, qpsExported %f", handleExported.Value(), connExported.Value(), timeExported.Value(), qpsExported.Value()))
}

// MonitorOn starts up an HTTP monitor on port.
func MonitorOn(port int) {
	go func() {
		http.HandleFunc("/monitor", monitorHander)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			logger.Errorln(err)
			return
		}
	}()
}

func addTotalConn(delta int64) {
	connExported.Add(delta)
	calculateQPS()
}

func addTotalHandle() {
	handleExported.Add(1)
	calculateQPS()
}

func addTotalTime(seconds float64) {
	timeExported.Add(seconds)
	calculateQPS()
}

func calculateQPS() {
	totalConn, err := strconv.ParseInt(connExported.String(), 10, 64)
	if err != nil {
		logger.Errorln(err)
		return
	}

	totalTime, err := strconv.ParseFloat(timeExported.String(), 64)
	if err != nil {
		logger.Errorln(err)
		return
	}

	totalHandle, err := strconv.ParseInt(handleExported.String(), 10, 64)
	if err != nil {
		logger.Errorln(err)
		return
	}

	if float64(totalConn)*totalTime != 0 {
		// take the average time per worker go-routine
		qps := float64(totalHandle) / (float64(totalConn) * (totalTime / float64(WorkerPoolInstance().Size())))
		qpsExported.Set(qps)
	}
}
