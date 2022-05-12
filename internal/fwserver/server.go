package fwserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/internal/logging"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// Server implements the framework provider server. Protocol specific
// implementations wrap this handling along with calling all request and
// response type conversions.
type Server struct {
	Provider tfsdk.Provider

	// dataSourceSchemas is the cached DataSource Schemas for RPCs that need to
	// convert configuration data from the protocol. If not found, it will be
	// fetched from the DataSourceType.GetSchema() method.
	dataSourceSchemas map[string]*tfsdk.Schema

	// dataSourceSchemasDiags is the cached Diagnostics obtained while populating
	// dataSourceSchemas. This is to ensure any warnings or errors are also
	// returned appropriately when fetching dataSourceSchemas.
	dataSourceSchemasDiags diag.Diagnostics

	// dataSourceSchemasMutex is a mutex to protect concurrent dataSourceSchemas
	// access from race conditions.
	dataSourceSchemasMutex sync.Mutex

	// dataSourceTypes is the cached DataSourceTypes for RPCs that need to
	// access data sources. If not found, it will be fetched from the
	// Provider.GetDataSources() method.
	dataSourceTypes map[string]tfsdk.DataSourceType

	// dataSourceTypesDiags is the cached Diagnostics obtained while populating
	// dataSourceTypes. This is to ensure any warnings or errors are also
	// returned appropriately when fetching dataSourceTypes.
	dataSourceTypesDiags diag.Diagnostics

	// dataSourceTypesMutex is a mutex to protect concurrent dataSourceTypes
	// access from race conditions.
	dataSourceTypesMutex sync.Mutex

	// providerSchema is the cached Provider Schema for RPCs that need to
	// convert configuration data from the protocol. If not found, it will be
	// fetched from the Provider.GetSchema() method.
	providerSchema *tfsdk.Schema

	// providerSchemaDiags is the cached Diagnostics obtained while populating
	// providerSchema. This is to ensure any warnings or errors are also
	// returned appropriately when fetching providerSchema.
	providerSchemaDiags diag.Diagnostics

	// providerSchemaMutex is a mutex to protect concurrent providerSchema
	// access from race conditions.
	providerSchemaMutex sync.Mutex
}

// DataSourceSchema returns the Schema associated with the DataSourceType for
// the given type name.
func (s *Server) DataSourceSchema(ctx context.Context, typeName string) (*tfsdk.Schema, diag.Diagnostics) {
	dataSourceSchemas, diags := s.DataSourceSchemas(ctx)

	dataSourceSchema, ok := dataSourceSchemas[typeName]

	if !ok {
		diags.AddError(
			"Data Source Schema Not Found",
			fmt.Sprintf("No data source type named %q was found in the provider to fetch the schema. ", typeName)+
				"This is always an issue in the Terraform Provider SDK used to implement the provider and should be reported to the provider developers.",
		)

		return nil, diags
	}

	return dataSourceSchema, diags
}

// DataSourceSchemas returns the map of DataSourceType Schemas. The results are
// cached on first use.
func (s *Server) DataSourceSchemas(ctx context.Context) (map[string]*tfsdk.Schema, diag.Diagnostics) {
	logging.FrameworkTrace(ctx, "Checking DataSourceSchemas lock")
	s.dataSourceSchemasMutex.Lock()
	defer s.dataSourceSchemasMutex.Unlock()

	if s.dataSourceSchemas != nil {
		return s.dataSourceSchemas, s.dataSourceSchemasDiags
	}

	dataSourceTypes, diags := s.DataSourceTypes(ctx)

	s.dataSourceSchemas = map[string]*tfsdk.Schema{}
	s.dataSourceSchemasDiags = diags

	if s.dataSourceSchemasDiags.HasError() {
		return s.dataSourceSchemas, s.dataSourceSchemasDiags
	}

	for dataSourceTypeName, dataSourceType := range dataSourceTypes {
		logging.FrameworkTrace(ctx, "Found data source type", map[string]interface{}{logging.KeyDataSourceType: dataSourceTypeName})

		logging.FrameworkDebug(ctx, "Calling provider defined DataSourceType GetSchema", map[string]interface{}{logging.KeyDataSourceType: dataSourceTypeName})
		schema, diags := dataSourceType.GetSchema(ctx)
		logging.FrameworkDebug(ctx, "Called provider defined DataSourceType GetSchema", map[string]interface{}{logging.KeyDataSourceType: dataSourceTypeName})

		s.dataSourceSchemasDiags.Append(diags...)

		if s.dataSourceSchemasDiags.HasError() {
			return s.dataSourceSchemas, s.dataSourceSchemasDiags
		}

		s.dataSourceSchemas[dataSourceTypeName] = &schema
	}

	return s.dataSourceSchemas, s.dataSourceSchemasDiags
}

// DataSourceType returns the DataSourceType for a given type name.
func (s *Server) DataSourceType(ctx context.Context, typeName string) (tfsdk.DataSourceType, diag.Diagnostics) {
	dataSourceTypes, diags := s.DataSourceTypes(ctx)

	dataSourceType, ok := dataSourceTypes[typeName]

	if !ok {
		diags.AddError(
			"Data Source Type Not Found",
			fmt.Sprintf("No data source type named %q was found in the provider.", typeName),
		)

		return nil, diags
	}

	return dataSourceType, diags
}

// DataSourceTypes returns the map of DataSourceTypes. The results are cached
// on first use.
func (s *Server) DataSourceTypes(ctx context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	logging.FrameworkTrace(ctx, "Checking DataSourceTypes lock")
	s.dataSourceTypesMutex.Lock()
	defer s.dataSourceTypesMutex.Unlock()

	if s.dataSourceTypes != nil {
		return s.dataSourceTypes, s.dataSourceTypesDiags
	}

	logging.FrameworkDebug(ctx, "Calling provider defined Provider GetDataSources")
	s.dataSourceTypes, s.dataSourceTypesDiags = s.Provider.GetDataSources(ctx)
	logging.FrameworkDebug(ctx, "Called provider defined Provider GetDataSources")

	return s.dataSourceTypes, s.dataSourceTypesDiags
}

// ProviderSchema returns the Schema associated with the Provider. The Schema
// and Diagnostics are cached on first use.
func (s *Server) ProviderSchema(ctx context.Context) (*tfsdk.Schema, diag.Diagnostics) {
	logging.FrameworkTrace(ctx, "Checking ProviderSchema lock")
	s.providerSchemaMutex.Lock()
	defer s.providerSchemaMutex.Unlock()

	if s.providerSchema != nil {
		return s.providerSchema, s.providerSchemaDiags
	}

	logging.FrameworkDebug(ctx, "Calling provider defined Provider GetSchema")
	providerSchema, diags := s.Provider.GetSchema(ctx)
	logging.FrameworkDebug(ctx, "Called provider defined Provider GetSchema")

	s.providerSchema = &providerSchema
	s.providerSchemaDiags = diags

	return s.providerSchema, s.providerSchemaDiags
}