//  This file is part of the eliona project.
//  Copyright © 2025 LEICOM iTEC AG. All Rights Reserved.
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

package eliona

import (
	"fmt"

	api "github.com/eliona-smart-building-assistant/go-eliona-api-client/v2"
	"github.com/eliona-smart-building-assistant/go-eliona/client"
	"github.com/eliona-smart-building-assistant/go-utils/common"
)

func GetDashboard(projectId string) (api.Dashboard, error) {
	dashboard := api.Dashboard{}
	dashboard.Name = "Demo"
	dashboard.ProjectId = projectId
	dashboard.Widgets = []api.Widget{}

	devices, _, err := client.NewClient().AssetsAPI.
		GetAssets(client.AuthenticationContext()).
		AssetTypeName("demo_asset").
		ProjectId(projectId).
		Execute()
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("fetching devices: %v", err)
	}

	widgetSequence := int32(0)
	for _, device := range devices {
		fmt.Println(device)
		var widgetData []api.WidgetData
		widgetData = append(widgetData, api.WidgetData{
			ElementSequence: nullableInt32(1),
			AssetId:         device.Id,
			Data: map[string]interface{}{
				"aggregatedDataField":  "avg",
				"aggregatedDataRaster": "M30",
				"aggregatedDataType":   "heap",
				"attribute":            "value",
				"description":          device.Name.Get(),
				"key":                  "",
				"seq":                  0,
				"subtype":              "input",
			},
		})
		dashboard.Widgets = append(dashboard.Widgets, api.Widget{
			WidgetTypeName: "CombinedTrends",
			AssetId:        device.Id,
			Sequence:       nullableInt32(widgetSequence),
			Details: map[string]any{
				"size":     1,
				"timespan": 7,
			},
			Data: widgetData,
		})
		widgetSequence++
	}

	return dashboard, nil
}

func nullableInt32(val int32) api.NullableInt32 {
	return *api.NewNullableInt32(common.Ptr[int32](val))
}
