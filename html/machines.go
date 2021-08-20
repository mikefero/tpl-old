package html

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikefero/tpl/db"
)

var reMachineName *regexp.Regexp
var reMachineFeatures *regexp.Regexp

func getMachineName(name string) string {
	return strings.TrimSpace(reMachineName.ReplaceAllString(name, ""))
}

func getMachineFeatures(id sql.NullInt64) string {
	var features string
	if id.Valid {
		features = db.GetFeatures(int(id.Int64))
		features = strings.ReplaceAll(features, ",", " / ")
		features = reMachineFeatures.ReplaceAllString(features, "")
	}

	return strings.TrimSpace(features)
}

func getMachineFeatureColor(id sql.NullInt64) string {
	var featuresColor string
	features := getMachineFeatures(id)
	if len(features) > 0 {
		if strings.Contains(features, "Vault") {
			featuresColor = "#900C3F"
		} else if strings.Contains(features, "Limited") {
			featuresColor = "#6600FF"
		} else if strings.Contains(features, "Premium") {
			featuresColor = "#FF5733"
		} else if strings.Contains(features, "Pro") {
			featuresColor = "#C70039"
		}
	}

	return featuresColor
}

func getMachineYear(timestamp sql.NullInt64) int {
	var year int
	if timestamp.Valid {
		date := time.Unix(timestamp.Int64, 0)
		year, _, _ = date.Date()
	}

	return year
}

func getMachineImageURL(uuid sql.NullString) string {
	var imageUrl = "http://www.thepinballlounge.com/pb/wp_0fa0cf0b/images/img165275761bbe29f98e.gif"
	if uuid.Valid {
		imageUrl = "https://img.opdb.org/" + uuid.String + "-medium.jpg"
	}

	return imageUrl
}

func handleMachines(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "machines.tmpl", gin.H{
		"title":       "Available Pinball Machines",
		"description": "Available pinball machines at The Pinball Lounge in Ovideo, Florida",
		"machines":    db.GetAllActiveMachines(),
	})
}
