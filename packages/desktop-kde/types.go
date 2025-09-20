package main

// KDEConfig represents a KDE configuration setting
type KDEConfig struct {
	File  string
	Group string
	Key   string
	Value string
	Desc  string
}

// WidgetInfo holds information about KDE Plasma widgets
type WidgetInfo struct {
	Name        string
	PackageName string
	Description string
	Category    string
}
