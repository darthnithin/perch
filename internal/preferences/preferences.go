// Package preferences stores explicit user defaults, never credentials or
// submission actions. Flags remain the final authority for each invocation.
package preferences

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"perch/internal/domain"
	"perch/internal/timewarning"
)

type Values struct {
	TimeWarnings  *string `json:"time_warnings,omitempty"`
	Version       int     `json:"schema_version"`
	Host          *string `json:"host,omitempty"`
	Theme         *string `json:"theme,omitempty"`
	ColorMode     *string `json:"color_mode,omitempty"`
	ReducedMotion *bool   `json:"reduced_motion,omitempty"`
	Mouse         *bool   `json:"mouse,omitempty"`
}

func DefaultStore() (Store, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: filepath.Join(root, "perch", "preferences")}, nil
}

func (s Store) Path() string { return filepath.Join(s.Dir, "settings.json") }

func (v *Values) Set(key, value string) error {
	switch key {
	case "host":
		if value == "" || len(value) > 512 || strings.HasPrefix(value, "-") || strings.ContainsAny(value, " \t\r\n") || domain.Label(value) != value {
			return fmt.Errorf("host must be an SSH destination or config alias, such as user@hb.ucsc.edu")
		}
		v.Host = &value
	case "time-warnings":
		policy, err := timewarning.Parse(value)
		if err != nil {
			return err
		}
		value = policy.String()
		v.TimeWarnings = &value
	case "theme":
		if value != "warm" && value != "graphite" {
			return fmt.Errorf("theme must be warm or graphite")
		}
		v.Theme = &value
	case "color-mode":
		if value != "auto" && value != "truecolor" && value != "256" {
			return fmt.Errorf("color-mode must be auto, truecolor or 256")
		}
		v.ColorMode = &value
	case "reduced-motion", "mouse":
		if value != "true" && value != "false" {
			return fmt.Errorf("%s must be true or false", key)
		}
		b, _ := strconv.ParseBool(value)
		if key == "mouse" {
			v.Mouse = &b
		} else {
			v.ReducedMotion = &b
		}
	default:
		return fmt.Errorf("unknown preference %q; choose host, theme, color-mode, reduced-motion, mouse or time-warnings", key)
	}
	return nil
}

func (v *Values) Unset(key string) error {
	switch key {
	case "time-warnings":
		v.TimeWarnings = nil
	case "host":
		v.Host = nil
	case "theme":
		v.Theme = nil
	case "color-mode":
		v.ColorMode = nil
	case "reduced-motion":
		v.ReducedMotion = nil
	case "mouse":
		v.Mouse = nil
	default:
		return fmt.Errorf("unknown preference %q", key)
	}
	return nil
}

// Flags contains only saved overrides; absent values keep built-in defaults.
func (v Values) Flags() map[string]string {
	out := map[string]string{}
	if v.TimeWarnings != nil {
		out["time-warnings"] = *v.TimeWarnings
	}
	if v.Host != nil {
		out["host"] = *v.Host
	}
	if v.Theme != nil {
		out["theme"] = *v.Theme
	}
	if v.ColorMode != nil {
		out["color-mode"] = *v.ColorMode
	}
	if v.ReducedMotion != nil {
		out["reduced-motion"] = strconv.FormatBool(*v.ReducedMotion)
	}
	if v.Mouse != nil {
		out["no-mouse"] = strconv.FormatBool(!*v.Mouse)
	}
	return out
}

func decode(raw []byte) (Values, error) {
	v := Values{Version: 1}
	if len(raw) == 0 {
		return v, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	v.Version = 0
	if err := decoder.Decode(&v); err != nil {
		return v, fmt.Errorf("invalid preferences JSON: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || v.Version != 1 {
		return v, fmt.Errorf("preferences require one JSON object with schema_version 1")
	}
	// Duplicate keys are likely editing mistakes and could hide a destination.
	d := json.NewDecoder(bytes.NewReader(raw))
	d.Token()
	seen := map[string]bool{}
	for d.More() {
		token, _ := d.Token()
		key, ok := token.(string)
		if !ok || seen[key] {
			return v, fmt.Errorf("duplicate or invalid preferences key")
		}
		seen[key] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return v, err
		}
	}
	valid := Values{Version: 1}
	if v.TimeWarnings != nil {
		if err := valid.Set("time-warnings", *v.TimeWarnings); err != nil {
			return v, err
		}
	}
	if v.Host != nil {
		if err := valid.Set("host", *v.Host); err != nil {
			return v, err
		}
	}
	if v.Theme != nil {
		if err := valid.Set("theme", *v.Theme); err != nil {
			return v, err
		}
	}
	if v.ColorMode != nil {
		if err := valid.Set("color-mode", *v.ColorMode); err != nil {
			return v, err
		}
	}
	return v, nil
}
