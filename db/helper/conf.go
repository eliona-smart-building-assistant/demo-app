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

package dbhelper

import (
	"context"
	"database/sql"
	appmodel "demo/v2/app/model"
	dbgen "demo/v2/db/generated"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/eliona-smart-building-assistant/go-eliona/v2/app"
	"github.com/eliona-smart-building-assistant/go-eliona/v2/frontend"
	"github.com/eliona-smart-building-assistant/go-utils/common"
	"github.com/eliona-smart-building-assistant/go-utils/log"
	"github.com/google/uuid"
)

var ErrBadRequest = errors.New("bad request")
var ErrNotFound = errors.New("not found")

func InsertConfig(ctx context.Context, config appmodel.Configuration) (appmodel.Configuration, error) {
	dbConfig, err := toDbConfig(ctx, config)
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("creating DB config from App config: %v", err)
	}
	if err := dbConfig.InsertG(ctx, boil.Infer()); err != nil {
		return appmodel.Configuration{}, fmt.Errorf("inserting DB config: %v", err)
	}
	return config, nil
}

func UpsertConfig(ctx context.Context, config appmodel.Configuration) (appmodel.Configuration, error) {
	dbConfig, err := toDbConfig(ctx, config)
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("creating DB config from App config: %v", err)
	}
	if err := dbConfig.UpsertG(ctx, true, []string{"id"}, boil.Blacklist("id"), boil.Infer()); err != nil {
		return appmodel.Configuration{}, fmt.Errorf("inserting DB config: %v", err)
	}
	return config, nil
}

func GetConfig(ctx context.Context, configID int64) (appmodel.Configuration, error) {
	dbConfig, err := dbgen.FindConfigurationG(ctx, configID)
	if errors.Is(err, sql.ErrNoRows) {
		return appmodel.Configuration{}, ErrNotFound
	}
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("fetching config from database: %v", err)
	}
	appConfig, err := toAppConfig(dbConfig)
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("creating App config from DB config: %v", err)
	}
	return appConfig, nil
}

func GetTenantConfig(ctx context.Context, configID int64, tenantId uuid.UUID) (appmodel.Configuration, error) {
	dbConfig, err := dbgen.Configurations(
		dbgen.ConfigurationWhere.ID.EQ(configID),
		dbgen.ConfigurationWhere.TenantID.EQ(tenantId.String()),
	).OneG(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return appmodel.Configuration{}, ErrNotFound
	}
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("fetching config from database: %v", err)
	}
	appConfig, err := toAppConfig(dbConfig)
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("creating App config from DB config: %v", err)
	}
	return appConfig, nil
}

func DeleteConfig(ctx context.Context, configID int64, tenantId uuid.UUID) error {
	// First, verify that the configuration exists and belongs to the specified tenant
	configExists, err := dbgen.Configurations(
		dbgen.ConfigurationWhere.ID.EQ(configID),
		dbgen.ConfigurationWhere.TenantID.EQ(tenantId.String()),
	).ExistsG(ctx)
	if err != nil {
		return fmt.Errorf("checking config existence: %v", err)
	}
	if !configExists {
		return ErrNotFound
	}

	// Delete assets that belong to this configuration
	if _, err := dbgen.Assets(
		dbgen.AssetWhere.ConfigurationID.EQ(configID),
	).DeleteAllG(ctx); err != nil {
		return fmt.Errorf("deleting assets from database: %v", err)
	}

	// Delete the configuration that matches both configID and tenantId
	count, err := dbgen.Configurations(
		dbgen.ConfigurationWhere.ID.EQ(configID),
		dbgen.ConfigurationWhere.TenantID.EQ(tenantId.String()),
	).DeleteAllG(ctx)
	if err != nil {
		return fmt.Errorf("deleting config from database: %v", err)
	}
	if count > 1 {
		return fmt.Errorf("shouldn't happen: deleted more (%v) configs by ID and tenant", count)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func toDbConfig(ctx context.Context, appConfig appmodel.Configuration) (dbConfig dbgen.Configuration, err error) {
	dbConfig.TenantID = appConfig.TenantId.String()

	dbConfig.ID = appConfig.Id
	dbConfig.SiteID = null.StringFrom(appConfig.SiteID)
	dbConfig.RefreshInterval = appConfig.RefreshInterval
	dbConfig.RequestTimeout = appConfig.RequestTimeout
	af, err := json.Marshal(appConfig.AssetFilter)
	if err != nil {
		return dbgen.Configuration{}, fmt.Errorf("marshalling assetFilter: %v", err)
	}
	dbConfig.AssetFilter = af
	dbConfig.Active = appConfig.Active
	dbConfig.Enable = appConfig.Enable

	env := frontend.GetEnvironment(ctx)
	if env != nil {
		dbConfig.UserID = env.UserId
	}

	return dbConfig, nil
}

func toAppConfig(dbConfig *dbgen.Configuration) (appConfig appmodel.Configuration, err error) {
	var apikey_err error

	tenantUUID, err := uuid.Parse(dbConfig.TenantID)
	if err != nil {
		return appmodel.Configuration{}, fmt.Errorf("parsing tenant ID as UUID: %v", err)
	}

	appConfig.ApiKey, apikey_err = app.GetApiKey("demo", GetDB(), tenantUUID)
	if apikey_err != nil {
		log.Fatal("conf", "api key not found for tenant %s in DB for: %v", dbConfig.TenantID, apikey_err)
	}

	appConfig.Id = dbConfig.ID
	appConfig.TenantId = tenantUUID
	appConfig.SiteID = dbConfig.SiteID.String
	appConfig.Enable = dbConfig.Enable
	appConfig.RefreshInterval = dbConfig.RefreshInterval
	appConfig.RequestTimeout = dbConfig.RequestTimeout
	var af [][]appmodel.FilterRule
	if err := json.Unmarshal(dbConfig.AssetFilter, &af); err != nil {
		return appmodel.Configuration{}, fmt.Errorf("unmarshalling assetFilter: %v", err)
	}
	appConfig.AssetFilter = af
	appConfig.Active = dbConfig.Active
	appConfig.UserId = dbConfig.UserID
	return appConfig, nil
}

func GetConfigs(ctx context.Context) ([]appmodel.Configuration, error) {
	dbConfigs, err := dbgen.Configurations().AllG(ctx)
	if err != nil {
		return nil, err
	}
	var appConfigs []appmodel.Configuration
	for _, dbConfig := range dbConfigs {
		ac, err := toAppConfig(dbConfig)
		if err != nil {
			return nil, fmt.Errorf("creating App config from DB config: %v", err)
		}
		appConfigs = append(appConfigs, ac)
	}
	return appConfigs, nil
}

func GetTenantConfigs(ctx context.Context, tenantId uuid.UUID) ([]appmodel.Configuration, error) {
	dbConfigs, err := dbgen.Configurations(
		dbgen.ConfigurationWhere.TenantID.EQ(tenantId.String()),
	).AllG(ctx)
	if err != nil {
		return nil, err
	}
	var appConfigs []appmodel.Configuration
	for _, dbConfig := range dbConfigs {
		ac, err := toAppConfig(dbConfig)
		if err != nil {
			return nil, fmt.Errorf("creating App config from DB config: %v", err)
		}
		appConfigs = append(appConfigs, ac)
	}
	return appConfigs, nil
}

func SetConfigActiveState(ctx context.Context, config appmodel.Configuration, state bool) (int64, error) {
	return dbgen.Configurations(
		dbgen.ConfigurationWhere.ID.EQ(config.Id),
	).UpdateAllG(ctx, dbgen.M{
		dbgen.ConfigurationColumns.Active: state,
	})
}

func SetAllConfigsInactive(ctx context.Context) (int64, error) {
	return dbgen.Configurations().UpdateAllG(ctx, dbgen.M{
		dbgen.ConfigurationColumns.Active: false,
	})
}

func InsertAsset(ctx context.Context, config appmodel.Configuration, globalAssetID string, assetId int32, providerId string, isRoot bool) error {
	dbAsset := dbgen.Asset{
		ConfigurationID: config.Id,
		GlobalAssetID:   globalAssetID,
		AssetID:         null.Int32From(assetId),
		ProviderID:      providerId,
		IsRoot:          isRoot,
	}
	return dbAsset.UpsertG(ctx, true, []string{dbgen.AssetColumns.ProviderID}, boil.Blacklist("id"), boil.Infer())
}

func GetAssetId(ctx context.Context, config appmodel.Configuration, globalAssetID string) (*int32, error) {
	dbAsset, err := dbgen.Assets(
		dbgen.AssetWhere.ConfigurationID.EQ(config.Id),
		dbgen.AssetWhere.GlobalAssetID.EQ(globalAssetID),
	).AllG(ctx)
	if err != nil || len(dbAsset) == 0 {
		return nil, err
	}
	return common.Ptr(dbAsset[0].AssetID.Int32), nil
}

func toAppAsset(dbAsset dbgen.Asset, config appmodel.Configuration) appmodel.Asset {
	return appmodel.Asset{
		ID:            dbAsset.ID,
		Config:        config,
		ProjectID:     dbAsset.ProjectID,
		GlobalAssetID: dbAsset.GlobalAssetID,
		ProviderID:    dbAsset.ProviderID,
		AssetID:       dbAsset.AssetID.Int32,
	}
}

func GetAssetById(assetId int32) (appmodel.Asset, error) {
	//Not used anywhere, but probably will need to support tenantId if used in future
	asset, err := dbgen.FindAssetG(context.Background(), int64(assetId))
	if err != nil {
		return appmodel.Asset{}, fmt.Errorf("fetching asset: %v", err)
	}
	if !asset.AssetID.Valid {
		return appmodel.Asset{}, fmt.Errorf("shouldn't happen: assetID is nil")
	}
	c, err := asset.Configuration().OneG(context.Background())
	if errors.Is(err, sql.ErrNoRows) {
		return appmodel.Asset{}, ErrNotFound
	}
	if err != nil {
		return appmodel.Asset{}, fmt.Errorf("fetching configuration: %v", err)
	}
	config, err := toAppConfig(c)
	if err != nil {
		return appmodel.Asset{}, fmt.Errorf("translating configuration: %v", err)
	}
	return toAppAsset(*asset, config), nil
}

func GetRootAssets(tenantId uuid.UUID) ([]appmodel.Asset, error) {
	assets, err := dbgen.Assets(
		dbgen.AssetWhere.IsRoot.EQ(true),
	).AllG(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fetching root assets: %v", err)
	}

	return FilterAssetsByTenant(assets, tenantId)
}

func GetAllDevices(tenantId uuid.UUID) ([]appmodel.Asset, error) {
	assets, err := dbgen.Assets(
		dbgen.AssetWhere.IsRoot.EQ(false),
	).AllG(context.Background())
	if err != nil {
		return nil, fmt.Errorf("fetching assets for tenant: %v", err)
	}

	return FilterAssetsByTenant(assets, tenantId)
}

func FilterAssetsByTenant(assets dbgen.AssetSlice, tenantId uuid.UUID) ([]appmodel.Asset, error) {
	appAssets := make([]appmodel.Asset, 0)
	for _, asset := range assets {
		c, err := asset.Configuration().OneG(context.Background())
		if errors.Is(err, sql.ErrNoRows) {
			continue // Skip assets without configuration
		}
		if err != nil {
			return nil, fmt.Errorf("fetching configuration: %v", err)
		}
		// Only include assets whose configuration belongs to the specified tenant
		if c.TenantID == tenantId.String() {
			config, err := toAppConfig(c)
			if err != nil {
				return nil, fmt.Errorf("translating configuration: %v", err)
			}
			appAssets = append(appAssets, toAppAsset(*asset, config))
		}
	}
	return appAssets, nil
}

func ParseTenantIdFromEnv(ctx context.Context) (uuid.UUID, error) {
	env := frontend.GetEnvironment(ctx)
	if env == nil {
		return uuid.UUID{}, fmt.Errorf("missing environment JWT")
	}
	parsed, err := uuid.Parse(env.TenantId)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("tenant isn't a valid UUID: %s", env.TenantId)
	}
	return parsed, err
}
