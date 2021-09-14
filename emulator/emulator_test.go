package emulator_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	//"google3/third_party/cloud_spanner_emulator/public/go/emulator"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"github.com/cloudspannerecosystem/harbourbridge/emulator"

	dbadminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	instancepb "google.golang.org/genproto/googleapis/spanner/admin/instance/v1"
)

const (
	testProjectID  = "test-project"
	testInstanceID = "test-instance"
	testDBName     = "test-database"
)

var schemaDDL = []string{
	`CREATE TABLE Users (
	   ID   INT64 NOT NULL,
		 Name STRING(MAX),
		 Age  INT64
	 ) PRIMARY KEY (ID)`,
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCreateDatabase(t *testing.T) {
	ctx := context.Background()
	emu, err := emulator.Start(ctx, emulator.Options{})
	if err != nil {
		t.Fatalf("Failed to bring up in-process cloud spanner emulator: %v", err)
	}
	defer emu.Stop()

	dbURI, err := setup(ctx, emu)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(ctx, emu)
	want := fmt.Sprintf("projects/%v/instances/%v/databases/%v", testProjectID, testInstanceID, testDBName)
	if dbURI != want {
		t.Errorf("Wanted %v, but got %v", want, dbURI)
	}
}

func TestDMLWrites(t *testing.T) {
	ctx := context.Background()
	emu, err := emulator.Start(ctx, emulator.Options{})
	if err != nil {
		t.Fatalf("Failed to bring up in-process cloud spanner emulator: %v", err)
	}
	defer emu.Stop()

	dbURI, err := setup(ctx, emu)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(ctx, emu)

	client, err := spanner.NewClient(ctx, dbURI, emu.ClientOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	// Use ReadWriteTransaction.Update to execute a DML statement.
	_, err = client.ReadWriteTransaction(ctx, func(ctx context.Context, tx *spanner.ReadWriteTransaction) error {
		count, err := tx.Update(ctx, spanner.Statement{
			SQL: `Insert INTO Users (ID, Name, Age) VALUES (2, "Eduard", 27)`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("row count: got %d, want 1", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func setup(ctx context.Context, emu *emulator.Emulator) (string, error) {
	// Create a new instance in emulator.
	instanceAdmin, err := instance.NewInstanceAdminClient(ctx, emu.ClientOptions()...)
	if err != nil {
		return "", fmt.Errorf("Failed to create an instance admin client for emulator: %v", err)
	}

	instanceURI, err := createInstance(ctx, instanceAdmin, testProjectID, testInstanceID)
	if err != nil {
		return "", fmt.Errorf("Failed to create an instance in emulator: %v", err)
	}
	// Create a new database in emulator.
	databaseAdmin, err := database.NewDatabaseAdminClient(ctx, emu.ClientOptions()...)
	if err != nil {
		return "", fmt.Errorf("Failed to create an database admin client for emulator: %v", err)
	}

	dbURI, err := createDatabase(ctx, databaseAdmin, instanceURI, testDBName, schemaDDL)
	if err != nil {
		return "", fmt.Errorf("Failed to create a database in emulator: %v", err)
	}
	return dbURI, nil
}

func cleanup(ctx context.Context, emu *emulator.Emulator) error {
	instanceAdmin, err := instance.NewInstanceAdminClient(ctx, emu.ClientOptions()...)
	if err != nil {
		return fmt.Errorf("Failed to create an instance admin client for emulator: %v", err)
	}
	err = deleteInstance(ctx, instanceAdmin, testProjectID, testInstanceID)
	if err != nil {
		fmt.Println(" couldnt clean up", err)
		return fmt.Errorf("Failed to delete instance in emulator: %v", err)
	}
	fmt.Println("cleaned up")
	return nil
}

func createInstance(ctx context.Context, instanceAdmin *instance.InstanceAdminClient, projectID, instanceID string) (string, error) {
	projectURI := fmt.Sprintf("projects/%v", projectID)
	instanceURI := fmt.Sprintf("%v/instances/%v", projectURI, instanceID)
	op, err := instanceAdmin.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     projectURI,
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			DisplayName: instanceID,
			NodeCount:   1,
		},
	})
	if err != nil {
		return "", fmt.Errorf("cannot create instance %v: %v", instanceURI, err)
	}
	if _, err = op.Wait(ctx); err != nil {
		return "", fmt.Errorf("cannot create instance %v: %v", instanceURI, err)
	}
	return instanceURI, nil
}

func createDatabase(ctx context.Context, databaseAdmin *database.DatabaseAdminClient, instanceURI, databaseID string, extraStatements []string) (string, error) {
	dbURI := fmt.Sprintf("%v/databases/%v", instanceURI, databaseID)
	op, err := databaseAdmin.CreateDatabase(ctx, &dbadminpb.CreateDatabaseRequest{
		Parent:          instanceURI,
		CreateStatement: "CREATE DATABASE `" + databaseID + "`",
		ExtraStatements: extraStatements,
	})
	if err != nil {
		return "", fmt.Errorf("cannot create testing DB %v: %v", dbURI, err)
	}
	if _, err = op.Wait(ctx); err != nil {
		return "", fmt.Errorf("cannot create testing DB %v: %v", dbURI, err)
	}
	return dbURI, nil
}

func deleteInstance(ctx context.Context, instanceAdmin *instance.InstanceAdminClient, projectID, instanceID string) error {
	projectURI := fmt.Sprintf("projects/%v", projectID)
	instanceURI := fmt.Sprintf("%v/instances/%v", projectURI, instanceID)
	err := instanceAdmin.DeleteInstance(ctx, &instancepb.DeleteInstanceRequest{
		Name: instanceURI,
	})
	if err != nil {
		return fmt.Errorf("cannot delete instance %v: %v", instanceURI, err)
	}
	return nil
}
