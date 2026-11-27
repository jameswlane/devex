package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// receiptPrefix keys the receipt DevEx writes for an application it installs.
const receiptPrefix = "installed-app-receipt:"

// ReceiptKey is the datastore key of an application's install receipt.
func ReceiptKey(name string) string {
	return receiptPrefix + strings.ToLower(strings.TrimSpace(name))
}

// Receipt records what DevEx installed, as the native package manager
// reported it right after the install.
type Receipt struct {
	Manager     string    `json:"manager"`
	Channel     string    `json:"channel,omitempty"`
	Packages    []string  `json:"packages"`
	Version     string    `json:"version,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	// MachineID is the salted machine hash Machine.ID reports, so records
	// copied from another machine can be recognized.
	MachineID string `json:"machine_id"`
	ManagedBy string `json:"managed_by"`
}

// Writer is the part of the DevEx datastore Record writes.
type Writer interface {
	Set(key, value string) error
}

// RecordOptions configures Record; zero values use the native query, the
// current time and /etc/machine-id.
type RecordOptions struct {
	Query         Query
	Now           func() time.Time
	MachineIDPath string
}

// Record writes the install receipt for an application DevEx just installed.
// A package manager that cannot be queried leaves the version empty; only a
// canceled context or a datastore failure is an error.
func Record(ctx context.Context, repo Writer, app types.CrossPlatformApp, options RecordOptions) error {
	if options.Query == nil {
		options.Query = NativeQuery
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	config := app.GetOSConfig()
	item := Item{Name: app.Name, Manager: "unknown", Packages: []string{}, State: StateUnknown}
	if config.InstallMethod != "" {
		item.Manager = config.InstallMethod
	}
	if err := inspect(ctx, &item, config, options.Query); err != nil {
		return err
	}
	receipt := Receipt{
		Manager: item.Manager, Channel: config.ChannelName(), Packages: item.Packages, InstalledAt: options.Now().UTC(),
		MachineID: machineID(options.MachineIDPath), ManagedBy: "devex",
	}
	if item.State == StateInstalled {
		receipt.Version = item.Version
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if err := repo.Set(ReceiptKey(app.Name), string(data)); err != nil {
		return fmt.Errorf("record install receipt for %s: %w", app.Name, err)
	}
	return nil
}

// ReceiptGetter is the minimal datastore interface ReadReceipt needs.
type ReceiptGetter interface {
	Get(key string) (string, error)
}

// ReadReceipt reads and decodes the install receipt recorded for name.
func ReadReceipt(repo ReceiptGetter, name string) (Receipt, bool) {
	if repo == nil {
		return Receipt{}, false
	}
	data, err := repo.Get(ReceiptKey(name))
	if err != nil || data == "" {
		return Receipt{}, false
	}
	var receipt Receipt
	if json.Unmarshal([]byte(data), &receipt) != nil {
		return Receipt{}, false
	}
	return receipt, true
}

func readReceipt(repo Reader, name string) (Receipt, bool) {
	return ReadReceipt(repo, name)
}

// compareReceipt fills the recorded fields of an item and reports drift
// between the receipt and the machine: a different installed version, or a
// receipt written on another machine.
func compareReceipt(repo Reader, item *Item, currentMachine string) []Drift {
	receipt, ok := readReceipt(repo, item.Name)
	if !ok {
		return nil
	}
	item.RecordedVersion = receipt.Version
	if !receipt.InstalledAt.IsZero() {
		installedAt := receipt.InstalledAt
		item.InstalledAt = &installedAt
	}
	var drift []Drift
	if receipt.MachineID != "" && receipt.MachineID != "unavailable" && currentMachine != "unavailable" && receipt.MachineID != currentMachine {
		drift = append(drift, Drift{App: item.Name, Kind: "host_changed", Detail: "recorded on another machine (" + receipt.MachineID + "); the DevEx records were copied or restored here"})
	}
	if item.State == StateInstalled && receipt.Version != "" && item.Version != "" && item.Version != receipt.Version {
		drift = append(drift, Drift{App: item.Name, Kind: "changed", Detail: "version " + receipt.Version + " was installed by DevEx; " + item.Manager + " now reports " + item.Version})
	}
	return drift
}
