package cli

import (
	"errors"
	"fmt"
	"os"
)

func runMigrate() {
	if errorValue := runMigrateArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runMigrateArguments(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "fleet-id" {
		return errors.New("usage: internkim migrate fleet-id [--new-fleet-id <id>] [--node <number>] [--host <ip>] [--remote-ssh]")
	}
	return runMigrateFleetID(commandControlArguments(arguments[1:]))
}

func runMigrateFleetID(arguments []string) error {
	configuration := loadConfig()
	target := resolveCommandTarget(arguments)
	oldFleetID := loadOrCreateFleetID(target.stateDir)
	if oldFleetID == "" {
		return errors.New("saved fleet_id not found; run setup once before migrating")
	}
	newFleetID := commandArgumentValue(arguments, "--new-fleet-id", "")
	if newFleetID == "" {
		newFleetID = randomFleetID()
	}
	if oldFleetID == newFleetID {
		return errors.New("new fleet id is the same as current fleet id")
	}
	connection, isRemote, errorValue := resolveDeviceSSHConnection(configuration, target)
	if errorValue != nil {
		return errorValue
	}
	target.host = connection.host
	target.useRemoteSSH = isRemote
	printCommandTargetEvidence(target)

	response, errorValue := registerFleetIDMigration(
		configuration,
		oldFleetID,
		newFleetID,
		loadNodeID(target.stateDir),
		loadOrCreateNodeKey(target.stateDir),
		loadOrCreateFleetSecret(target.stateDir),
		remoteSetupAdminEmail(target.stateDir),
	)
	if errorValue != nil {
		return errorValue
	}
	if response.registeredFleetID() != newFleetID {
		return fmt.Errorf("migration did not return requested fleet id: requested %s, got %s", newFleetID, response.registeredFleetID())
	}
	saveRegistrationResponse(target.stateDir, response)
	updateRemoteDeviceRegistration(connection, configuration, response)
	fmt.Printf("Migrated fleet ID: %s -> %s\n", oldFleetID, response.registeredFleetID())
	if response.AliasURL != "" {
		fmt.Printf("Alias: %s\n", response.AliasURL)
	}
	fmt.Printf("URL: %s\n", response.publicURL())
	return nil
}
