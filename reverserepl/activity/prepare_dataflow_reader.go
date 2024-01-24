// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package activity

import (
	"context"
	"fmt"
	"slices"

	dataflowaccessor "github.com/GoogleCloudPlatform/spanner-migration-tool/accessors/dataflow"
	storageaccessor "github.com/GoogleCloudPlatform/spanner-migration-tool/accessors/storage"
	dataflowutils "github.com/GoogleCloudPlatform/spanner-migration-tool/accessors/utils/dataflow"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/common/constants"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/common/utils"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/logger"
	resource "github.com/GoogleCloudPlatform/spanner-migration-tool/reverserepl/resource"
)

type PrepareDataflowReaderInput struct {
	SmtJobId                string
	ChangeStreamName        string
	InstanceId              string
	DatabaseId              string
	SpannerProjectId        string
	SessionFilePath         string
	SourceShardsFilePath    string
	MetadataInstance        string
	MetadataDatabase        string
	GcsOutputDirectory      string
	StartTimestamp          string
	EndTimestamp            string
	WindowDuration          string
	FiltrationMode          string
	MetadataTableSuffix     string
	SkipDirectoryName       string
	ShardingCustomJarPath   string
	ShardingCustomClassName string
	TuningCfg               string
	SpannerLocation         string
}

type PrepareDataflowReaderOutput struct {
	JobId string
}

type PrepareDataflowReader struct {
	Input  *PrepareDataflowReaderInput
	Output *PrepareDataflowReaderOutput
	DfA    dataflowaccessor.DataflowAccessor
	SA     storageaccessor.StorageAccessor
}

// Launches the reader dataflow job.
func (p *PrepareDataflowReader) Transaction(ctx context.Context) error {
	input := p.Input
	readerTuningCfg, err := dataflowutils.UnmarshalDataflowTuningConfig(ctx, p.SA, input.TuningCfg)
	if err != nil {
		return fmt.Errorf("error reading reader tuning config %s: %v", input.TuningCfg, err)
	}
	logger.Log.Debug(fmt.Sprintf("readerTuningCfg: %+v", readerTuningCfg))
	validateUpdateReaderTuningCfg(&readerTuningCfg, input.SpannerProjectId, input.SpannerLocation, input.SmtJobId)
	logger.Log.Debug(fmt.Sprintf("Updated readerTuningCfg: %+v", readerTuningCfg))

	params := map[string]string{
		"changeStreamName":     input.ChangeStreamName,
		"instanceId":           input.InstanceId,
		"databaseId":           input.DatabaseId,
		"spannerProjectId":     input.SpannerProjectId,
		"metadataInstance":     input.MetadataInstance,
		"metadataDatabase":     input.MetadataDatabase,
		"gcsOutputDirectory":   input.GcsOutputDirectory,
		"sessionFilePath":      input.SessionFilePath,
		"sourceShardsFilePath": input.SourceShardsFilePath,
		"endTimestamp":         input.EndTimestamp,
		"windowDuration":       input.WindowDuration,
		"filtrationMode":       input.FiltrationMode,
		"metadataTableSuffix":  input.MetadataTableSuffix,
		"skipDirectoryName":    input.SkipDirectoryName,
		"startTimestamp":       input.StartTimestamp,
		"runIdentifier":        input.SmtJobId,
		"runMode":              constants.RR_READER_REGULAR_MODE,
	}
	// Cannot send empty strings since the template expects GCS file paths.
	if input.ShardingCustomJarPath != "" {
		params["shardingCustomJarPath"] = input.ShardingCustomJarPath
		params["shardingCustomClassName"] = input.ShardingCustomClassName
	}
	dfLaunchReq, err := dataflowutils.GetDataflowLaunchRequest(params, readerTuningCfg)
	if err != nil {
		return err
	}
	dfJobId, err := resource.CreateDataflowSMTResource(ctx, p.DfA, input.SmtJobId, dfLaunchReq)
	if err != nil {
		return err
	}
	logger.Log.Info(fmt.Sprintf("Launched reader job with id: %s", dfJobId))
	logger.Log.Info(fmt.Sprintf("\nEquivalent gCloud command for job %s:\n%s\n\n", dfLaunchReq.LaunchParameter.JobName, dataflowutils.GetGcloudDataflowCommand(dfLaunchReq)))
	p.Output.JobId = dfJobId
	return nil
}

func (p *PrepareDataflowReader) Compensation(ctx context.Context) error {
	return nil
}

func validateUpdateReaderTuningCfg(cfg *dataflowaccessor.DataflowTuningConfig, spannerProjectId, spannerLocation, smtJobId string) {
	if cfg.ProjectId == "" {
		cfg.ProjectId = spannerProjectId
	}
	if cfg.JobName == "" {
		cfg.JobName = fmt.Sprintf("smt-reverse-replication-reader-%s", utils.GenerateHashStr())
	}
	if cfg.Location == "" {
		cfg.Location = spannerLocation
	}
	if cfg.MaxWorkers == 0 {
		cfg.MaxWorkers = 50
	}
	if cfg.NumWorkers == 0 {
		cfg.NumWorkers = 5
	}
	if cfg.MachineType == "" {
		cfg.MachineType = "n1-standard-2"
	}
	cfg.AdditionalUserLabels["smt-reverse-replication-reader"] = smtJobId
	if cfg.GcsTemplatePath == "" {
		cfg.GcsTemplatePath = constants.REVERSE_REPLICATION_READER_TEMPLATE_PATH
	}
	if cfg.AdditionalExperiments == nil {
		cfg.AdditionalExperiments = []string{"use_runner_v2"}
	} else if !slices.Contains(cfg.AdditionalExperiments, "use_runner_v2") {
		cfg.AdditionalExperiments = append(cfg.AdditionalExperiments, "use_runner_v2")
	}
	cfg.EnableStreamingEngine = true
}
