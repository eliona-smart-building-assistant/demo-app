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

package eliona

import (
	"context"
	appmodel "demo/v2/app/model"
	conf "demo/v2/db/helper"
	"fmt"

	"github.com/eliona-smart-building-assistant/go-eliona/v2/utils"
	"github.com/eliona-smart-building-assistant/go-utils/common"
)

type ExampleDevice struct {
	ID   string `eliona:"id" subtype:"info"`
	Name string `eliona:"name,filterable" subtype:"info"`

	LocationalParentGAI string
	FunctionalParentGAI string

	Config *appmodel.Configuration
}

func (d *ExampleDevice) AdheresToFilter(filter [][]appmodel.FilterRule) (bool, error) {
	f := appFilterToCommonFilter(filter)
	fp, err := utils.StructToMap(d)
	if err != nil {
		return false, fmt.Errorf("converting struct to map: %v", err)
	}
	adheres, err := common.Filter(f, fp)
	if err != nil {
		return false, err
	}
	return adheres, nil
}

func (d *ExampleDevice) GetName() string {
	return d.Name
}

func (d *ExampleDevice) GetDescription() string {
	return ""
}

func (d *ExampleDevice) GetAssetType() string {
	return "demo_asset"
}

func (d *ExampleDevice) GetGAI() string {
	return d.GetAssetType() + "_" + d.ID
}

func (d *ExampleDevice) GetAssetID() (*int32, error) {
	return conf.GetAssetId(context.Background(), *d.Config, d.GetGAI())
}

func (d *ExampleDevice) SetAssetID(assetID int32) error {
	if err := conf.InsertAsset(context.Background(), *d.Config, d.GetGAI(), assetID, d.ID, false); err != nil {
		return fmt.Errorf("inserting asset to config db: %v", err)
	}
	return nil
}

func (d *ExampleDevice) GetSiteID() string {
	return d.Config.SiteID
}

func (a *ExampleDevice) GetLocationalParentGAI() string {
	return a.LocationalParentGAI
}

func (a *ExampleDevice) GetFunctionalParentGAI() string {
	return a.FunctionalParentGAI
}

type Root struct {
	LocationalParentGAI string
	FunctionalParentGAI string

	Config *appmodel.Configuration
}

func (r *Root) GetName() string {
	return "demo"
}

func (r *Root) GetDescription() string {
	return "Root asset for Demo devices"
}

func (r *Root) GetAssetType() string {
	return "demo_root"
}

func (r *Root) GetGAI() string {
	return r.GetAssetType()
}

func (r *Root) GetAssetID() (*int32, error) {
	return conf.GetAssetId(context.Background(), *r.Config, r.GetGAI())
}

func (r *Root) SetAssetID(assetID int32) error {
	if err := conf.InsertAsset(context.Background(), *r.Config, r.GetGAI(), assetID, "", true); err != nil {
		return fmt.Errorf("inserting asset to config db: %v", err)
	}
	return nil
}

func (r *Root) GetSiteID() string {
	return r.Config.SiteID
}

func (a *Root) GetLocationalParentGAI() string {
	return a.LocationalParentGAI
}

func (a *Root) GetFunctionalParentGAI() string {
	return a.FunctionalParentGAI
}

//

func appFilterToCommonFilter(input [][]appmodel.FilterRule) [][]common.FilterRule {
	result := make([][]common.FilterRule, len(input))
	for i := 0; i < len(input); i++ {
		result[i] = make([]common.FilterRule, len(input[i]))
		for j := 0; j < len(input[i]); j++ {
			result[i][j] = common.FilterRule{
				Parameter: input[i][j].Parameter,
				Regex:     input[i][j].Regex,
			}
		}
	}
	return result
}
