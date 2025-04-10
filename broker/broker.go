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

package broker

import (
	appmodel "demo/app/model"
	"demo/eliona"
	"fmt"

	"github.com/eliona-smart-building-assistant/go-eliona/asset"
)

func TestAuthentication(config appmodel.Configuration) error {
	if config.ApiKey != "12345" {
		return fmt.Errorf("Incorrect API key!")
	}
	return nil
}

func GetDevices(config appmodel.Configuration) ([]asset.AssetWithParentReferences, error) {
	if err := TestAuthentication(config); err != nil {
		return nil, err
	}

	assets := []asset.AssetWithParentReferences{}
	root := eliona.Root{
		Config: &config,
	}
	assets = append(assets, &root)
	device := eliona.ExampleDevice{
		Name:                fmt.Sprintf("Device from config %v", config.Id),
		ID:                  fmt.Sprintf("%v", config.Id),
		LocationalParentGAI: root.GetGAI(),
		FunctionalParentGAI: root.GetGAI(),
		Config:              &config,
	}
	assets = append(assets, &device)

	return assets, nil
}
