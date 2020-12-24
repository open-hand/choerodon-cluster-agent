package cron

import (
	"fmt"
	"github.com/robfig/cron/v3"
)

var CronJob = make(map[string]func())

func StartCron(errChan chan<- error) {
	c := cron.New()

	for spec, f := range CronJob {
		_, err := c.AddFunc(spec, f)
		if err != nil {
			errChan <- fmt.Errorf("failed to add cron job: %s", err.Error())
		}
	}

	c.Start()
}

func AddCronJob(spec string, job func()) {
	CronJob[spec] = job
}
