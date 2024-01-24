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
package reverserepl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	spanneraccessor "github.com/GoogleCloudPlatform/spanner-migration-tool/accessors/spanner"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/activity"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/common/constants"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/common/utils"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/dao"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/logger"
	rractivity "github.com/GoogleCloudPlatform/spanner-migration-tool/reverserepl/activity"
	"github.com/GoogleCloudPlatform/spanner-migration-tool/webv2/helpers"
)

func validateAndUpdateJobData(ctx context.Context, request *JobData, uuid string) (err error) {
	request.IsSMTBucketRequired = true
	request.SmtBucketName = fmt.Sprintf("smt-rr-gcs-%s", uuid)
	if strings.HasPrefix(request.SessionFilePath, constants.GCS_FILE_PREFIX) && strings.HasPrefix(request.SourceConnectionConfig, constants.GCS_FILE_PREFIX) && request.GcsDataDirectory != "" {
		request.IsSMTBucketRequired = false
		request.SmtBucketName = ""
	}
	if request.InstanceId == "" {
		return fmt.Errorf("found empty InstanceId which is a required parameter")
	}
	if request.DatabaseId == "" {
		return fmt.Errorf("found empty DatabaseId which is a required parameter")
	}
	if request.SessionFilePath == "" {
		return fmt.Errorf("found empty SessionFilePath which is a required parameter")
	} else if !strings.HasPrefix(request.SessionFilePath, constants.GCS_FILE_PREFIX) {
		request.SessionFileGcsPath = fmt.Sprintf("%s%s/session.json", constants.GCS_FILE_PREFIX, request.SmtBucketName)
	} else {
		request.SessionFileGcsPath = request.SessionFilePath
	}
	if request.SourceConnectionConfig == "" {
		return fmt.Errorf("found empty SourceConnectionConfig which is a required parameter")
	} else if !strings.HasPrefix(request.SourceConnectionConfig, constants.GCS_FILE_PREFIX) {
		request.SourceConnectionConfigGcsPath = fmt.Sprintf("%s%s/source-connection-config.json", constants.GCS_FILE_PREFIX, request.SmtBucketName)
	} else {
		request.SourceConnectionConfigGcsPath = request.SourceConnectionConfig
	}
	if request.SpannerProjectId == "" {
		return fmt.Errorf("found empty SpannerProjectId which is a required parameter")
	}
	if request.JobName == "" {
		request.JobName = fmt.Sprintf("smt-job-%s", uuid)
	}
	if request.SourceType == "" {
		request.SourceType = constants.MYSQL
	}
	if request.SourceType != constants.MYSQL {
		return fmt.Errorf("%s is not a valid source type for reverse replication. Only supported source type is mysql", request.SourceType)
	}
	if request.MetadataInstance == "" {
		request.MetadataInstance = request.InstanceId
	}
	if request.MetadataDatabase == "" {
		request.MetadataDatabase = fmt.Sprintf("smt-rr-metadata-%s", uuid)
	}
	if request.GcsDataDirectory == "" {
		request.GcsDataDirectory = fmt.Sprintf("gs://smt-rr-gcs-%s/reverse-replication/data", uuid)
	} else if !strings.HasPrefix(request.GcsDataDirectory, constants.GCS_FILE_PREFIX) {
		return fmt.Errorf("invalid gcs path for GcsDataDirectory: %s", request.GcsDataDirectory)
	}
	if request.ChangeStreamName == "" {
		request.ChangeStreamName = fmt.Sprintf("smt-rr-cs-%s", uuid)
	}
	if request.FiltrationMode == "" {
		request.FiltrationMode = constants.RR_READER_FILTER_FWD
	} else if !utils.Contains([]string{constants.RR_READER_FILTER_FWD, constants.RR_READER_FILTER_NONE}, request.FiltrationMode) {
		return fmt.Errorf("found filtrationMode %s, only allowed values are [%s, %s]", request.FiltrationMode, constants.RR_READER_FILTER_FWD, constants.RR_READER_FILTER_NONE)
	}
	if request.TimerInterval < 1 {
		request.TimerInterval = 1
	}
	if request.WindowDuration == "" {
		request.WindowDuration = "10s"
	}

	if request.ShardingCustomJarPath != "" && request.ShardingCustomClassName == "" {
		return fmt.Errorf("found non-empty value for ShardingCustomJarPath, but empty value for ShardingCustomClassName")
	}
	if request.ShardingCustomJarPath == "" && request.ShardingCustomClassName != "" {
		return fmt.Errorf("found non-empty value for ShardingCustomClassName, but empty value for ShardingCustomJarPath")
	}
	if request.ShardingCustomJarPath != "" && !strings.HasPrefix(request.ShardingCustomJarPath, constants.GCS_FILE_PREFIX) {
		return fmt.Errorf("please specify a valid GCS path for ShardingCustomJarPath, starting with gs://")
	}
	// Replace '-' with '_' since hyphens are not allowed in cs names.
	request.ChangeStreamName = strings.Replace(request.ChangeStreamName, "-", "_", -1)

	request.SpannerLocation, err = spanneraccessor.GetSpannerLeaderLocation(ctx, fmt.Sprintf("projects/%s/instances/%s", request.SpannerProjectId, request.InstanceId))
	return err
}

// CreateWorkflows sets up the data flow job and required resources for a reverse replication pipeline.
func CreateWorkflow(ctx context.Context, request JobData) error {
	// Move to initialization to CLI layer.
	_ = logger.InitializeLogger("DEBUG")
	defer logger.Log.Sync()

	logger.Log.Info("Creating reverse replication pipeline.")
	logger.Log.Debug(fmt.Sprintf("Received Create Reverse Replication job request: %+v\n", request))
	uuid := utils.GenerateHashStr()
	err := validateAndUpdateJobData(ctx, &request, uuid)
	if err != nil {
		return fmt.Errorf("error in validateCreateRequest: %v", err)
	}
	logger.Log.Debug(fmt.Sprintf("Updated job request: %+v\n", request))

	// Check or create the internal metadata database for all flows.
	helpers.CheckOrCreateMetadataDb(request.SpannerProjectId, request.InstanceId)
	smtMetadataDBURI := helpers.GetSpannerUri(request.SpannerProjectId, request.InstanceId)
	// Init dao client.
	_, err = dao.GetOrCreateClient(ctx, smtMetadataDBURI)
	if err != nil {
		return fmt.Errorf("error starting dao client: %v", err)
	}

	smtJobId := fmt.Sprintf("smt-job-%s", uuid)
	b, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error converting job data to string: %v", err)
	}
	jobData := string(b)
	activities := []activity.Activity{
		&rractivity.CreateSmtJobEntry{
			Input: &rractivity.CreateSmtJobEntryInput{
				SmtJobId:         smtJobId,
				JobName:          request.JobName,
				SpannerProjectId: request.SpannerProjectId,
				InstanceId:       request.InstanceId,
				DatabaseId:       request.DatabaseId,
				JobData:          jobData,
			},
		},
		&rractivity.PrepareGcsBucket{
			Input: &rractivity.PrepareGcsBucketInput{
				SmtJobId:               smtJobId,
				SmtBucketName:          request.SmtBucketName,
				SpannerProjectId:       request.SpannerProjectId,
				SpannerLocation:        request.SpannerLocation,
				SessionFilePath:        request.SessionFilePath,
				SourceConnectionConfig: request.SourceConnectionConfig,
				IsSMTBucketRequired:    request.IsSMTBucketRequired,
			},
		},
		&rractivity.PrepareChangeStream{
			Input: &rractivity.PrepareChangeStreamInput{
				SmtJobId:         smtJobId,
				ChangeStreamName: request.ChangeStreamName,
				DbURI:            fmt.Sprintf("projects/%s/instances/%s/databases/%s", request.SpannerProjectId, request.InstanceId, request.DatabaseId),
			},
			Output: &rractivity.PrepareChangeStreamOutput{},
		},
		&rractivity.PrepareMetadataDb{
			Input: &rractivity.PrepareMetadataDbInput{
				SmtJobId: smtJobId,
				DbURI:    fmt.Sprintf("projects/%s/instances/%s/databases/%s", request.SpannerProjectId, request.MetadataInstance, request.MetadataDatabase),
			},
			Output: &rractivity.PrepareMetadataDbOutput{},
		},
		&rractivity.PrepareDataflowReader{
			Input: &rractivity.PrepareDataflowReaderInput{
				SmtJobId:                smtJobId,
				ChangeStreamName:        request.ChangeStreamName,
				InstanceId:              request.InstanceId,
				DatabaseId:              request.DatabaseId,
				SpannerProjectId:        request.SpannerProjectId,
				SessionFilePath:         request.SessionFileGcsPath,
				SourceShardsFilePath:    request.SourceConnectionConfigGcsPath,
				MetadataInstance:        request.MetadataInstance,
				MetadataDatabase:        request.MetadataDatabase,
				GcsOutputDirectory:      request.GcsDataDirectory,
				StartTimestamp:          request.StartTimestamp,
				EndTimestamp:            request.EndTimestamp,
				WindowDuration:          request.WindowDuration,
				FiltrationMode:          request.FiltrationMode,
				MetadataTableSuffix:     request.MetadataTableSuffix,
				SkipDirectoryName:       request.SkipDirectoryName,
				ShardingCustomJarPath:   request.ShardingCustomJarPath,
				ShardingCustomClassName: request.ShardingCustomClassName,
				TuningCfg:               request.ReaderCfg,
				SpannerLocation:         request.SpannerLocation,
			},
			Output: &rractivity.PrepareDataflowReaderOutput{},
		},
		&rractivity.PrepareDataflowWriter{
			Input: &rractivity.PrepareDataflowWriterInput{
				SmtJobId:               smtJobId,
				SourceShardsFilePath:   request.SourceConnectionConfigGcsPath,
				SessionFilePath:        request.SessionFileGcsPath,
				SourceType:             request.SourceType,
				SourceDbTimezoneOffset: request.SourceDbTimezoneOffset,
				TimerInterval:          request.TimerInterval,
				StartTimestamp:         request.StartTimestamp,
				WindowDuration:         request.WindowDuration,
				GCSInputDirectoryPath:  request.GcsDataDirectory,
				SpannerProjectId:       request.SpannerProjectId,
				MetadataInstance:       request.MetadataInstance,
				MetadataDatabase:       request.MetadataDatabase,
				MetadataTableSuffix:    request.MetadataTableSuffix,
				TuningCfg:              request.WriterCfg,
				SpannerLocation:        request.SpannerLocation,
			},
			Output: &rractivity.PrepareDataflowWriterOutput{},
		},
		&rractivity.UpdateSmtJobEntry{
			Input: &rractivity.UpdateSmtJobEntryInput{
				SmtJobId: smtJobId,
				State:    "RUNNING",
			},
		},
	}
	for i, activity := range activities {
		if err := activity.Transaction(ctx); err != nil {
			// If a local transaction fails, execute the compensating actions for all previous steps
			// for i := len(s.Steps) - 1; i >= 0; i-- {
			//     if err := s.Steps[i].Compensate(); err != nil {
			//         return errors.New(fmt.Sprintf("failed to compensate for step %d: %v", i, err))
			//     }
			// }
			return fmt.Errorf("error executing activity #%d: %v", i, err)
		}
	}
	logger.Log.Info("Successfully launched reverse replication pipeline.")
	return nil
}
