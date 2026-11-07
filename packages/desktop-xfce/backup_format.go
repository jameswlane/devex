package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// XFCE backups are XML in the shape of xfconf's own per-channel files:
//
//	<xfce-config format="devex-xfce-backup" version="2">
//	  <channel name="xfwm4">
//	    <property name="/general/theme" type="string" value="Default"/>
//	    <property name="/general/workspace_names" type="array">
//	      <value type="string" value="Main"/>
//	    </property>
//	  </channel>
//	</xfce-config>
//
// type is the xfconf type name, so a restore can recreate a missing property
// with `-n -t <type>`. xfconf-query only reports types from 4.20 (`-p <prop>
// -v` prints "[type] value"); with older xfconf a scalar is recorded without a
// type and restored without -t, which keeps the existing property's type and
// fails, reported, if the property no longer exists. Values that cannot be
// recorded faithfully (an untyped multi-line value, such as an array printed by
// older xfconf) are skipped with a warning at backup time.
const (
	xfceBackupFormat  = "devex-xfce-backup"
	xfceBackupVersion = 2
)

type xfceBackup struct {
	XMLName  xml.Name      `xml:"xfce-config"`
	Format   string        `xml:"format,attr,omitempty"`
	Version  int           `xml:"version,attr,omitempty"`
	Channels []xfceChannel `xml:"channel"`
}

type xfceChannel struct {
	Name       string         `xml:"name,attr"`
	Properties []xfceProperty `xml:"property"`
}

type xfceProperty struct {
	Name   string      `xml:"name,attr"`
	Type   string      `xml:"type,attr,omitempty"`
	Value  *string     `xml:"value,attr"`
	Values []xfceValue `xml:"value"`
}

type xfceValue struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

// xfconfScalarTypes are the value types xfconf-query accepts for -t.
var xfconfScalarTypes = []string{"string", "int", "uint", "bool", "double", "uchar", "char", "int16", "uint16", "int64", "uint64"}

func xfconfQuery(args ...string) ([]byte, error) {
	return exec.Command("xfconf-query", args...).Output()
}

// parseChannelListing reads `xfconf-query -l`: the first line is the header,
// translated by locale, so it is skipped by position; channel names follow,
// indented.
func parseChannelListing(output string) []string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var channels []string
	for _, line := range lines[1:] {
		if channel := strings.TrimSpace(line); channel != "" {
			channels = append(channels, channel)
		}
	}
	return channels
}

// parsePropertyListing reads `xfconf-query -c <channel> -l`: one unindented
// property path per line. An empty channel prints a translated "contains no
// properties" sentence instead, which never starts with "/".
func parsePropertyListing(output string) []string {
	var properties []string
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if property := strings.TrimSpace(line); strings.HasPrefix(property, "/") {
			properties = append(properties, property)
		}
	}
	return properties
}

// readXFCEProperty reads a property's value and, where xfconf reports it, type.
func readXFCEProperty(channel, property string) (xfceProperty, error) {
	plain, err := xfconfQuery("-c", channel, "-p", property)
	if err != nil {
		return xfceProperty{}, err
	}
	verbose, err := xfconfQuery("-c", channel, "-p", property, "-v")
	if err != nil {
		return xfceProperty{}, err
	}
	return decodeXFCEProperty(property, string(plain), string(verbose))
}

// decodeXFCEProperty compares plain and verbose reads. A typed read is
// accepted only when it is exactly the plain read with "[type] " prefixes, so
// a value that merely looks typed is never misread.
func decodeXFCEProperty(name, plain, verbose string) (xfceProperty, error) {
	plain = strings.TrimSuffix(plain, "\n")
	verbose = strings.TrimSuffix(verbose, "\n")

	if valueType, value, ok := splitTyped(verbose); ok && value == plain {
		return xfceProperty{Name: name, Type: valueType, Value: &value}, nil
	}

	// An array prints a translated count line and a blank line, then one item
	// per line.
	plainLines, verboseLines := strings.Split(plain, "\n"), strings.Split(verbose, "\n")
	if len(plainLines) > 2 && len(plainLines) == len(verboseLines) &&
		plainLines[0] == verboseLines[0] && plainLines[1] == "" && verboseLines[1] == "" {
		values := make([]xfceValue, 0, len(plainLines)-2)
		for i := 2; i < len(plainLines); i++ {
			valueType, value, ok := splitTyped(verboseLines[i])
			if !ok || value != plainLines[i] {
				values = nil
				break
			}
			values = append(values, xfceValue{Type: valueType, Value: value})
		}
		if values != nil {
			return xfceProperty{Name: name, Type: "array", Values: values}, nil
		}
	}

	if plain == verbose && !strings.Contains(plain, "\n") {
		return xfceProperty{Name: name, Value: &plain}, nil
	}
	return xfceProperty{}, errors.New("value has no type information and spans several lines, so it cannot be restored faithfully")
}

func splitTyped(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "[") {
		return "", "", false
	}
	valueType, value, found := strings.Cut(line[1:], "] ")
	if !found || !slices.Contains(xfconfScalarTypes, valueType) {
		return "", "", false
	}
	return valueType, value, true
}

// parseXFCEBackup decodes and validates a backup before any setting is written.
func parseXFCEBackup(content []byte) (xfceBackup, error) {
	var backup xfceBackup
	decoder := xml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&backup); err != nil {
		return backup, fmt.Errorf("not an XFCE settings backup: %w", err)
	}
	// Decode reads one root element; reject another root or non-whitespace data.
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return backup, fmt.Errorf("invalid trailing XML: %w", err)
		}
		switch value := token.(type) {
		case xml.Comment:
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return backup, errors.New("unexpected content after XFCE backup")
			}
		default:
			return backup, errors.New("unexpected content after XFCE backup")
		}
	}
	if backup.Format != "" && backup.Format != xfceBackupFormat {
		return backup, fmt.Errorf("unsupported backup format %q", backup.Format)
	}
	if backup.Version > xfceBackupVersion {
		return backup, fmt.Errorf("backup version %d is newer than this plugin supports (%d)", backup.Version, xfceBackupVersion)
	}

	properties := 0
	seen := map[string]bool{}
	for _, channel := range backup.Channels {
		if channel.Name == "" || strings.HasPrefix(channel.Name, "-") || strings.ContainsFunc(channel.Name, isSpaceOrControl) {
			return backup, fmt.Errorf("invalid channel name %q", channel.Name)
		}
		for _, property := range channel.Properties {
			if err := validateXFCEProperty(property); err != nil {
				return backup, fmt.Errorf("channel %s: %w", channel.Name, err)
			}
			key := channel.Name + "\x00" + property.Name
			if seen[key] {
				return backup, fmt.Errorf("channel %s: property %s appears more than once", channel.Name, property.Name)
			}
			seen[key] = true
			properties++
		}
	}
	if properties == 0 {
		return backup, errors.New("no settings to restore; the backup has no properties (backups made before channel listing was fixed are empty), create a new one with 'devex desktop-xfce backup'")
	}
	return backup, nil
}

func validateXFCEProperty(property xfceProperty) error {
	if !strings.HasPrefix(property.Name, "/") || strings.ContainsFunc(property.Name, isSpaceOrControl) {
		return fmt.Errorf("invalid property name %q", property.Name)
	}
	switch {
	case property.Type == "array":
		if property.Value != nil || len(property.Values) == 0 {
			return fmt.Errorf("property %s: an array needs one or more <value> items and no value attribute", property.Name)
		}
		for _, item := range property.Values {
			if !slices.Contains(xfconfScalarTypes, item.Type) {
				return fmt.Errorf("property %s: unsupported array item type %q", property.Name, item.Type)
			}
		}
	case property.Type == "" || slices.Contains(xfconfScalarTypes, property.Type):
		if property.Value == nil || len(property.Values) > 0 {
			return fmt.Errorf("property %s: a single value needs a value attribute and no <value> items", property.Name)
		}
	default:
		return fmt.Errorf("property %s: unsupported type %q", property.Name, property.Type)
	}
	return nil
}

func isSpaceOrControl(r rune) bool {
	return r <= ' ' || r == 0x7f
}

// restoreArgs builds the xfconf-query argv for one validated property. Typed
// values pass -n with -t so a missing property is created; untyped values keep
// the existing property's type.
func restoreArgs(channel string, property xfceProperty) []string {
	args := []string{"-c", channel, "-p", property.Name}
	switch property.Type {
	case "":
		return append(args, "-s", *property.Value)
	case "array":
		args = append(args, "-n", "-a")
		for _, item := range property.Values {
			args = append(args, "-t", item.Type, "-s", item.Value)
		}
		return args
	default:
		return append(args, "-n", "-t", property.Type, "-s", *property.Value)
	}
}
