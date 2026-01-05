// Package epg provides parsing and filtering for EPG (Electronic Program Guide) XML data.
package epg

import (
	"encoding/xml"
	"fmt"
)

// TV represents the root element of an XMLTV EPG file.
type TV struct {
	XMLName  xml.Name    `xml:"tv"`
	Channels []Channel   `xml:"channel"`
	Programs []Programme `xml:"programme"`
}

// Channel represents a channel in the EPG.
type Channel struct {
	ID          string `xml:"id,attr"`
	DisplayName string `xml:"display-name"`
	Icon        Icon   `xml:"icon"`
}

// Icon represents a channel or programme icon.
type Icon struct {
	Src string `xml:"src,attr"`
}

// Programme represents a programme/show in the EPG.
type Programme struct {
	Channel     string `xml:"channel,attr"`
	Start       string `xml:"start,attr"`
	Stop        string `xml:"stop,attr"`
	Title       string `xml:"title"`
	Description string `xml:"desc"`
	Category    string `xml:"category,omitempty"`
}

// Parse parses EPG XML data into a TV structure.
func Parse(data []byte) (*TV, error) {
	var tv TV
	if err := xml.Unmarshal(data, &tv); err != nil {
		return nil, fmt.Errorf("failed to parse EPG XML: %w", err)
	}

	return &tv, nil
}

// Marshal serializes the TV structure to XML.
func Marshal(tv *TV) ([]byte, error) {
	data, err := xml.MarshalIndent(tv, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EPG XML: %w", err)
	}

	return append([]byte(xml.Header), data...), nil
}
