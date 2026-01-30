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
	apiserver "demo/v2/api/generated"
	apiservices "demo/v2/api/services"
	appmodel "demo/v2/app/model"
	"demo/v2/broker"
	dbhelper "demo/v2/db/helper"
	"demo/v2/eliona"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	api "github.com/eliona-smart-building-assistant/go-eliona-api-client/v3"
	"github.com/eliona-smart-building-assistant/go-eliona/v2/asset"
	"github.com/eliona-smart-building-assistant/go-eliona/v2/client"
	"github.com/eliona-smart-building-assistant/go-eliona/v2/frontend"
	"github.com/eliona-smart-building-assistant/go-utils/common"
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

		err := asset.InitAssetTypeFiles(client.ApiEndpointString(), config.ApiKey, "resources/asset-type-*.json")
		if err != nil {
			return
		}

		if !config.Active {
			dbhelper.SetConfigActiveState(context.Background(), config, true)
			log.Info("dbhelper", "Collecting initialized with Configuration %d:\n"+
				"SiteId: %s\n"+
				"Enable: %t\n"+
				"Refresh Interval: %d\n"+
				"Request Timeout: %d\n",
				config.Id,
				config.SiteID,
				config.Enable,
				config.RefreshInterval,
				config.RequestTimeout)
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
	configs, err := dbhelper.GetConfigs(context.Background())
	if err != nil {
		log.Fatal("app", "couldn't read configs from DB: %v", err)
	}
	for _, config := range configs {
		if err != nil {
			log.Fatal("conf", "api key not found for tenant %s in DB for: %v", config.TenantId, err)
		}
		go generateData(config)
	}
}

func generateData(config appmodel.Configuration) {
	assets, err := dbhelper.GetAllDevices(config.TenantId)
	if err != nil {
		log.Error("dbhelper", "getting assets: %v", err)
		return
	}
	for _, asset := range assets {
		value := generateSinWaveData()
		data := map[string]any{
			"value": value,
		}
		if err := eliona.UpsertData(config, asset.AssetID, data, time.Now(), api.INPUT); err != nil {
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
	configs, err := dbhelper.GetConfigs(context.Background())
	if err != nil {
		log.Fatal("app", "couldn't read configs from DB: %v", err)
	}
	for _, config := range configs {
		if err != nil {
			log.Fatal("conf", "api key not found for tenant %s in DB for: %v", config.TenantId, err)
		}
		go heartbeat(config)
	}
}

func heartbeat(config appmodel.Configuration) {
	roots, err := dbhelper.GetRootAssets(config.TenantId)
	if err != nil {
		log.Error("dbhelper", "getting root assets: %v", err)
		return
	}

	for _, root := range roots {
		err := eliona.UpsertData(config, root.AssetID, map[string]any{"status": appStatus}, time.Now(), api.STATUS)
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
