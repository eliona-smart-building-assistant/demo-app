//  This file is part of the Eliona project.
//  Copyright © 2024 IoTEC AG. All Rights Reserved.
//  ______ _ _
// |  ____| (_)
// | |__  | |_  ___  _ __   __ _
// |  __| | | |/ _ \| '_ \ / _` |
// | |____| | | (_) | | | | (_| |
// |______|_|_|\___/|_| |_|\__,_|
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING
//  BUT NOT LIMITED  TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
//  NON INFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
//  DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package app

import (
	"context"
	apiserver "demo-app/api/generated"
	apiservices "demo-app/api/services"
	appmodel "demo-app/app/model"
	"demo-app/broker"
	dbhelper "demo-app/db/helper"
	"demo-app/eliona"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	api "github.com/eliona-smart-building-assistant/go-eliona-api-client/v2"
	"github.com/eliona-smart-building-assistant/go-eliona/app"
	"github.com/eliona-smart-building-assistant/go-eliona/asset"
	"github.com/eliona-smart-building-assistant/go-eliona/dashboard"
	"github.com/eliona-smart-building-assistant/go-eliona/frontend"
	"github.com/eliona-smart-building-assistant/go-utils/common"
	"github.com/eliona-smart-building-assistant/go-utils/db"
	utilshttp "github.com/eliona-smart-building-assistant/go-utils/http"
	"github.com/eliona-smart-building-assistant/go-utils/log"
)

var appStatus = 0

const (
	statusOK = iota
	statusError
	statusFatal
)

func changeAppStatus(status int) {
	appStatus = status
	Heartbeat()
}

func Initialize() {
	ctx := context.Background()

	// Necessary to close used init resources
	conn := db.NewInitConnectionWithContextAndApplicationName(ctx, app.AppName())
	defer conn.Close(ctx)

	// Init the app before the first run.
	app.Init(conn, app.AppName(),
		app.ExecSqlFile("db/init.sql"),
		asset.InitAssetTypeFiles("resources/asset-types/*.json"),
		dashboard.InitWidgetTypeFiles("resources/widget-types/*.json"),
	)
}

var once sync.Once
var startTime time.Time

func CollectData() {
	configs, err := dbhelper.GetConfigs(context.Background())
	if err != nil {
		log.Fatal("dbhelper", "Couldn't read configs from DB: %v", err)
		changeAppStatus(statusFatal)
		return
	}
	if len(configs) == 0 {
		once.Do(func() {
			log.Info("dbhelper", "No configs in DB. Please configure the app in Eliona.")
		})
		return
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}

	for _, config := range configs {
		if !config.Enable {
			if config.Active {
				dbhelper.SetConfigActiveState(context.Background(), config, false)
			}
			continue
		}

		if !config.Active {
			dbhelper.SetConfigActiveState(context.Background(), config, true)
			log.Info("dbhelper", "Collecting initialized with Configuration %d:\n"+
				"Enable: %t\n"+
				"Refresh Interval: %d\n"+
				"Request Timeout: %d\n"+
				"Project IDs: %v\n",
				config.Id,
				config.Enable,
				config.RefreshInterval,
				config.RequestTimeout,
				config.ProjectIDs)
		}

		common.RunOnceWithParam(func(config appmodel.Configuration) {
			log.Info("main", "Collecting %d started.", config.Id)
			if err := collectResources(config); err != nil {
				changeAppStatus(statusError)
				return // Error is handled in the method itself.
			}
			log.Info("main", "Collecting %d finished.", config.Id)
			changeAppStatus(statusOK)

			time.Sleep(time.Second * time.Duration(config.RefreshInterval))
		}, config, config.Id)
	}
}

func collectResources(config appmodel.Configuration) error {
	devices, err := broker.GetDevices(config)
	if err != nil {
		log.Error("broker", "getting devices: %v", err)
		return err
	}
	if err := eliona.CreateAssets(config, devices); err != nil {
		log.Error("eliona", "creating assets: %v", err)
		return err
	}

	return nil
}

func GenerateData() {
	assets, err := dbhelper.GetAllDevices()
	if err != nil {
		log.Error("dbhelper", "getting assets: %v", err)
		return
	}
	for _, asset := range assets {
		value := generateSinWaveData()
		data := map[string]any{
			"value": value,
		}
		if err := eliona.UpsertData(asset.AssetID, data, time.Now(), api.SUBTYPE_INPUT); err != nil {
			log.Error("eliona", "upserting data. %v", err)
			return
		}
	}
	return
}

func generateSinWaveData() float64 {
	max := 100.0
	min := 0.0
	frequency := 0.01
	amplitude := (max - min) / 2
	offset := min + amplitude
	randomize := rand.Float64() * 10
	return amplitude*math.Sin(2*math.Pi*frequency*float64(time.Since(startTime))) + offset + randomize
}

// outputData implements passing output data to broker. Remove if not needed.
func outputData(asset appmodel.Asset, data map[string]interface{}) error {
	// Do the output magic here.
	return nil
}

func Heartbeat() {
	roots, err := dbhelper.GetRootAssets()
	if err != nil {
		log.Error("dbhelper", "getting root assets: %v", err)
		return
	}

	for _, root := range roots {
		err := eliona.UpsertData(root.AssetID, map[string]any{"status": appStatus}, time.Now(), api.SUBTYPE_STATUS)
		if err != nil {
			log.Error("eliona", "upserting data as heartbeat: %v", err)
			return
		}
	}
}

// ListenApi starts the API server and listen for requests
func ListenApi() {
	err := http.ListenAndServe(":"+common.Getenv("API_SERVER_PORT", "3000"),
		frontend.NewEnvironmentHandler(
			utilshttp.NewCORSEnabledHandler(
				apiserver.NewRouter(
					apiserver.NewConfigurationAPIController(apiservices.NewConfigurationAPIService()),
					apiserver.NewVersionAPIController(apiservices.NewVersionAPIService()),
					apiserver.NewCustomizationAPIController(apiservices.NewCustomizationAPIService()),
				))))
	log.Fatal("main", "API server: %v", err)
	changeAppStatus(statusFatal)
}
