package utils

import (
	"context"
	customoperations "minioclient/custom_operations"
	"minioclient/global"
	"time"

	"github.com/sirupsen/logrus"
)

func ConnectionHealthCheck(client *global.GlobalClient) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	statuscode, e := customoperations.HealthCheck(client, ctx)
	if e != nil || statuscode != 200 {
		logrus.Warnf("CONNECTION HEALTCHECK STATUS CODE: %d", statuscode)
		logrus.Warnf("CONNECTION HEALTCHECK ERROR: %s", e.Error())
		return false
	}

	return true
}
