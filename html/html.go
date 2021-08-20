package html

import (
	"html/template"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/mikefero/tpl/log"
)

func handleRoot(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title":       "The Pinball Lounge",
		"description": "The Pinball Lounge in Ovideo, Florida",
	})
}

func ListenAndServe() {
	log.Debug("initializing regular expressions")
	reMachineName = regexp.MustCompile(`(?i)\(.*\)`)
	reMachineFeatures = regexp.MustCompile(`(?i) play| edition| table | model| game`)
	log.Debug("regular expressions initialized")

	log.Debug("initializing gin router")
	router := gin.Default()
	router.SetFuncMap(template.FuncMap{
		"getMachineName":         getMachineName,
		"getMachineFeatures":     getMachineFeatures,
		"getMachineFeatureColor": getMachineFeatureColor,
		"getMachineYear":         getMachineYear,
		"getMachineImageURL":     getMachineImageURL,
	})
	log.Debug("gin router initialized")

	log.Debug("initializing assets and templates")
	router.Static("/bootstrap", "./html/assets/bootstrap-5.1.0-dist")
	router.LoadHTMLGlob("html/templates/*.tmpl")
	log.Debug("assets and templates initialized")

	log.Debug("initializing endpoints")
	router.GET("/", handleRoot)
	router.GET("/machines", handleMachines)
	log.Debug("endpoints initialized")

	log.Debug("starting gin router")
	router.Run(":8989")
	log.Debug("gin router started")
}
