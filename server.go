package main

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var (
	sessions   = make(map[string]string)
	sessionsMu sync.Mutex
	carts      = make(map[string][]part)
	cartsMu    sync.Mutex

	to = []string{"glennwarden@yahoo.com", "john@jjdev.com"}

	// to = []string{"john@jjdev.com"}
	senderPass = os.Getenv("SMTPPASS")
)

const (
	smtpServer = "smtp.gmail.com"
	smtpPort   = "587"
	senderMail = "john@jjdev.com"
)

type part interface {
	choices(string) []string
}

type Nipple struct {
	Size     string
	Quantity int
}

func (Nipple) choices(string) []string {
	return nippleSizes
}

var nippleSizes = func() []string {
	sizes := []string{"close"}
	for inches := 2; inches <= 12; inches++ {
		sizes = append(sizes, fmt.Sprintf("1″ × %d″", inches))
		if inches < 12 {
			sizes = append(sizes, fmt.Sprintf("1″ × %d½″", inches))
		}
	}
	return sizes
}()

type Pipe struct {
	ThicknessType string
	CoatingType   string
	EndType       string
	Size          string
	Feet          int
}

func (Pipe) choices(field string) []string {
	return pipeChoices[field]
}

type MechanicalTee struct {
	Size     string
	Quantity int
}

func (MechanicalTee) choices(string) []string {
	return mechanicalTeeSizes
}

type ScrewedTee struct {
	RunSize      string
	BullheadSize string
	Quantity     int
}

func (ScrewedTee) choices(field string) []string {
	return screwedTeeChoices[field]
}

var screwedTeeChoices = func() map[string][]string {
	outlets := []string{"2½″", "2″", "1½″", "1¼″", "1″", "½″"}
	var sizes []string
	for _, first := range outlets {
		for _, second := range outlets {
			sizes = append(sizes, first+" × "+second)
		}
	}
	return map[string][]string{
		"RunSize":      sizes,
		"BullheadSize": outlets,
	}
}()

type PipeStand struct {
	Size     string
	Quantity int
}

func (PipeStand) choices(string) []string {
	return pipeStandSizes
}

var pipeStandSizes = []string{
	"6″",
	"4″",
	"3″",
	"2½″",
	"2″",
	"1½″",
	"1¼″",
	"1″",
}

type GroovedCoupling struct {
	Type     string
	Size     string
	Quantity int
}

func (GroovedCoupling) choices(field string) []string {
	return groovedCouplingChoices[field]
}

var groovedCouplingChoices = map[string][]string{
	"Type": {"rigid", "flexible"},
	"Size": {"8″", "6″", "4″", "3″", "2½″", "2″", "1½″", "1¼″", "1″"},
}

type GroovedReducingCoupling struct {
	Size     string
	Quantity int
}

func (GroovedReducingCoupling) choices(string) []string {
	return groovedReducingCouplingSizes
}

var groovedReducingCouplingSizes = []string{
	"8″ × 6″",
	"6″ × 4″",
	"4″ × 3″",
	"4″ × 2½″",
	"4″ × 2″",
	"3″ × 2½″",
	"3″ × 2″",
	"2½″ × 2″",
	"2″ × 1¼″",
	"2″ × 1″",
	"2″ × ½″",
}

type GroovedFlangeAdapter struct {
	Size     string
	Quantity int
}

func (GroovedFlangeAdapter) choices(string) []string {
	return groovedFlangeAdapterSizes
}

var groovedFlangeAdapterSizes = []string{
	"12″", "10″", "8″", "6″", "4″", "3″", "2½″", "2″",
}

func formatPart(p part) string {
	value := reflect.ValueOf(p)
	partType := value.Type()
	var fields []string
	for index := range value.NumField() {
		field := partType.Field(index)
		if field.Tag.Get("optional") == "true" && value.Field(index).IsZero() {
			continue
		}
		fields = append(fields, fmt.Sprintf("%s: %v", field.Name, value.Field(index).Interface()))
	}
	return partType.Name() + ": " + strings.Join(fields, ", ")
}

func mapForm[T part](r *http.Request, p *T) map[string]error {
	errs := make(map[string]error)
	for field, value := range reflect.ValueOf(p).Elem().Fields() {
		input := r.Form.Get(field.Name)
		if field.Tag.Get("input") == "text" {
			input = strings.TrimSpace(input)
		}
		if input == "" {
			if field.Tag.Get("optional") != "true" {
				errs[field.Name] = errors.New("required")
			}
			continue
		}
		switch value.Kind() {
		case reflect.String:
			if field.Tag.Get("input") != "text" && !slices.Contains((*p).choices(field.Name), input) {
				errs[field.Name] = fmt.Errorf("invalid value %q provided", input)
				continue
			}
			value.SetString(input)
		case reflect.Int:
			number, err := parsePositiveInt(input)
			if err != nil {
				errs[field.Name] = err
				continue
			}
			value.SetInt(int64(number))
		}
	}
	return errs
}

func parsePositiveInt(text string) (int, error) {
	number, err := strconv.Atoi(text)
	if err != nil {
		return 0, errors.New("value must be a whole number within the supported range")
	}
	if number < 1 {
		return 0, errors.New("must be greater than 0")
	}
	return number, nil
}

var pipeChoices = map[string][]string{
	"ThicknessType": {
		"sch 10",
		"sch 40",
	},
	"CoatingType": {
		"regular",
		"galvanized",
	},
	"EndType": {
		"plain end",
		"threaded",
		"grooved",
	},
	"Size": {
		"8″",
		"6″",
		"4″",
		"3″",
		"2½″",
		"2″",
		"1½″",
		"1¼″",
		"1″",
	},
}

var mechanicalTeeSizes = []string{
	"8″ × 4″",
	"8″ × 3″",
	"8″ × 2½″",
	"8″ × 2″",
	"8″ × 1½″",
	"8″ × 1¼″",
	"8″ × 1″",
	"6″ × 4″",
	"6″ × 3″",
	"6″ × 2½″",
	"6″ × 2″",
	"6″ × 1½″",
	"6″ × 1¼″",
	"6″ × 1″",
	"6″ × ¾″",
	"4″ × 2½″",
	"4″ × 2″",
	"4″ × 1½″",
	"4″ × 1¼″",
	"4″ × 1″",
	"4″ × ¾″",
	"3″ × 2″",
	"3″ × 1½″",
	"3″ × 1¼″",
	"3″ × 1″",
	"3″ × ¾″",
	"2½″ × 1½″",
	"2½″ × 1¼″",
	"2½″ × 1″",
	"2½″ × ¾″",
	"2″ × 1¼″",
	"2″ × 1″",
	"2″ × ¾″",
	"2″ × ½″",
}

// Common size defaults for the new parts; existing part choices stay unchanged.
// CPVC: https://www.vikinggroupinc.com/sites/default/files/2025-05/Section%20S_April-2025.pdf
// ATR: https://www.eaton.com/us/en-us/catalog/support-systems/channel-nuts--u-bolts--atr-and-hardware.html
// Sprinklers: https://www.vikinggroupinc.com/sites/default/files/2024-09/022324.pdf
// Glue: https://weldon.com/products/weld-on-500-fire-sprinkler/
var (
	threadedFittingSizes = []string{"½″", "¾″", "1″", "1¼″", "1½″", "2″", "2½″", "3″", "4″"}
	cpvcSizes            = []string{"¾″", "1″", "1¼″", "1½″", "2″", "2½″", "3″"}
	copperSizes          = []string{"½″", "¾″", "1″", "1¼″", "1½″", "2″", "2½″", "3″", "4″"}
	sprinklerSizes       = []string{"½″", "¾″"}
	hardwareSizes        = []string{"¼″", "5⁄16″", "⅜″", "½″", "⅝″", "¾″", "⅞″", "1″"}
	rodLengths           = []string{"3′", "6′", "10′", "12′"}
	fastenerLengths      = []string{"1″", "1½″", "2″", "2½″", "3″", "4″", "6″"}
	glueSizes            = []string{"pint", "quart"}
	containerSizes       = []string{"½ pint", "pint", "quart", "gallon"}
)

type GroovedElbow struct {
	Size     string
	Angle    string
	Quantity int
}

func (GroovedElbow) choices(field string) []string {
	return map[string][]string{
		"Size":  pipeChoices["Size"],
		"Angle": []string{"90°", "45°"},
	}[field]
}

type GroovedTee struct {
	Size     string
	Quantity int
}

func (GroovedTee) choices(string) []string { return pipeChoices["Size"] }

type GroovedCross struct {
	Size     string
	Quantity int
}

func (GroovedCross) choices(string) []string { return pipeChoices["Size"] }

type GroovedCap struct {
	Size     string
	Quantity int
}

func (GroovedCap) choices(string) []string { return pipeChoices["Size"] }

type AddACap struct {
	Size     string
	Quantity int
}

func (AddACap) choices(string) []string { return pipeChoices["Size"] }

type TapCap struct {
	Size     string
	Quantity int
}

func (TapCap) choices(string) []string { return pipeChoices["Size"] }

type GroovedReducer struct {
	Size     string
	Quantity int
}

func (GroovedReducer) choices(string) []string { return groovedReducingCouplingSizes }

type ConcentricReducer struct {
	Size     string
	Quantity int
}

func (ConcentricReducer) choices(string) []string { return groovedReducingCouplingSizes }

type ThreadedElbow struct {
	Size     string
	Material string
	Finish   string
	Angle    string
	Quantity int
}

func (ThreadedElbow) choices(field string) []string {
	return map[string][]string{
		"Size":     threadedFittingSizes,
		"Material": []string{"cast iron", "ductile iron"},
		"Finish":   []string{"black", "galvanized"},
		"Angle":    []string{"90°", "45°"},
	}[field]
}

type ThreadedCoupling struct {
	Size     string
	Material string
	Finish   string
	Quantity int
}

func (ThreadedCoupling) choices(field string) []string {
	return map[string][]string{
		"Size":     threadedFittingSizes,
		"Material": []string{"cast iron", "ductile iron"},
		"Finish":   []string{"black", "galvanized"},
	}[field]
}

type ThreadedUnion struct {
	Size     string
	Material string
	Finish   string
	Quantity int
}

func (ThreadedUnion) choices(field string) []string {
	return map[string][]string{
		"Size":     threadedFittingSizes,
		"Material": []string{"cast iron", "ductile iron"},
		"Finish":   []string{"black", "galvanized"},
	}[field]
}

type ThreadedBushing struct {
	Size       string
	OutletSize string
	Material   string
	Finish     string
	Quantity   int
}

func (ThreadedBushing) choices(field string) []string {
	return map[string][]string{
		"Size":       threadedFittingSizes,
		"OutletSize": threadedFittingSizes,
		"Material":   []string{"cast iron", "ductile iron"},
		"Finish":     []string{"black", "galvanized"},
	}[field]
}

type CPVCPipe struct {
	Size string
	Feet int
}

func (CPVCPipe) choices(string) []string { return cpvcSizes }

type CPVCAdapter struct {
	Size       string
	Connection string
	Quantity   int
}

func (CPVCAdapter) choices(field string) []string {
	return map[string][]string{
		"Size":       cpvcSizes,
		"Connection": []string{"socket", "male threaded", "female threaded"},
	}[field]
}

type CPVCTee struct {
	Size     string
	Quantity int
}

func (CPVCTee) choices(string) []string { return cpvcSizes }

type CPVCElbow struct {
	Size     string
	Angle    string
	Quantity int
}

func (CPVCElbow) choices(field string) []string {
	return map[string][]string{
		"Size":  cpvcSizes,
		"Angle": []string{"90°", "45°"},
	}[field]
}

type CPVCCement struct {
	Size     string
	Quantity int
}

func (CPVCCement) choices(string) []string { return glueSizes }

type OnePartGlue struct {
	Size     string
	Quantity int
}

func (OnePartGlue) choices(string) []string { return glueSizes }

type SeamlessSteelPipe struct {
	Size     string
	Schedule string
	Feet     int
}

func (SeamlessSteelPipe) choices(field string) []string {
	return map[string][]string{
		"Size":     pipeChoices["Size"],
		"Schedule": []string{"10", "40"},
	}[field]
}

type CopperTubing struct {
	Size string
	Type string
	Feet int
}

func (CopperTubing) choices(field string) []string {
	return map[string][]string{
		"Size": copperSizes,
		"Type": []string{"K", "L", "M"},
	}[field]
}

type CopperFitting struct {
	Size       string
	Fitting    string
	Connection string
	Quantity   int
}

func (CopperFitting) choices(field string) []string {
	return map[string][]string{
		"Size":       copperSizes,
		"Fitting":    []string{"elbow", "tee", "coupling", "adapter", "reducer", "cap"},
		"Connection": []string{"sweat", "pro-press"},
	}[field]
}

type Gasket struct {
	Size     string
	Type     string
	Quantity int
}

func (Gasket) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
		"Type": []string{"EPDM", "tri-seal"},
	}[field]
}

type PipeLubricant struct {
	Size     string
	Quantity int
}

func (PipeLubricant) choices(string) []string { return containerSizes }

type CommercialSprinkler struct {
	Size        string
	KFactor     string `label:"K-factor (US)"`
	Model       string `input:"text" optional:"true"`
	Response    string
	Orientation string
	Coverage    string
	Quantity    int
}

func (CommercialSprinkler) choices(field string) []string {
	return map[string][]string{
		"Size":        sprinklerSizes,
		"KFactor":     []string{"2.8", "4.2", "5.6", "8.0", "11.2", "14.0", "16.8"},
		"Response":    []string{"quick response (QR)", "standard response (SR)"},
		"Orientation": []string{"upright", "pendent", "sidewall"},
		"Coverage":    []string{"standard", "extended"},
	}[field]
}

type ResidentialSprinkler struct {
	Size     string
	KFactor  string `label:"K-factor (US)"`
	Model    string `input:"text" optional:"true"`
	Type     string
	Quantity int
}

func (ResidentialSprinkler) choices(field string) []string {
	return map[string][]string{
		"Size":    sprinklerSizes,
		"KFactor": []string{"3.0", "3.7", "3.8", "4.0", "4.2", "4.3", "4.9", "5.2", "5.8", "7.4"},
		"Type":    []string{"flat concealed pendent", "recessed pendent", "horizontal sidewall"},
	}[field]
}

type ESFRSprinkler struct {
	Size     string
	KFactor  string `label:"K-factor (US)"`
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (ESFRSprinkler) choices(field string) []string {
	return map[string][]string{
		"Size":    []string{"¾″", "1″"},
		"KFactor": []string{"14.0", "16.8", "22.4", "25.2", "28.0"},
	}[field]
}

type CMSASprinkler struct {
	Size     string
	KFactor  string `label:"K-factor (US)"`
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (CMSASprinkler) choices(field string) []string {
	return map[string][]string{
		"Size":    []string{"¾″", "1″"},
		"KFactor": []string{"11.2", "19.6"},
	}[field]
}

type DryPendent struct {
	Size     string
	KFactor  string `label:"K-factor (US)"`
	Inches   int
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (DryPendent) choices(field string) []string {
	return map[string][]string{
		"Size":    sprinklerSizes,
		"KFactor": []string{"5.6", "8.0", "11.2"},
	}[field]
}

type Escutcheon struct {
	Size     string
	Type     string
	Quantity int
}

func (Escutcheon) choices(field string) []string {
	return map[string][]string{
		"Size": sprinklerSizes,
		"Type": []string{"recessed", "flat", "adjustable"},
	}[field]
}

type HeadGuard struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HeadGuard) choices(field string) []string {
	if field == "Size" {
		return sprinklerSizes
	}
	return nil
}

type Baffle struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

// https://www.vikinggroupinc.com/sites/default/files/2025-01/012498_02.pdf
func (Baffle) choices(field string) []string {
	if field == "Size" {
		return []string{"2¾″", "3″", "4⅜″"}
	}
	return nil
}

type SprinklerWrench struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (SprinklerWrench) choices(string) []string { return nil }

type OSYGateValve struct {
	Size       string
	Connection string
	Quantity   int
}

func (OSYGateValve) choices(field string) []string {
	return map[string][]string{
		"Size":       pipeChoices["Size"],
		"Connection": []string{"threaded", "grooved", "flanged"},
	}[field]
}

type ButterflyValve struct {
	Size       string
	Connection string
	Quantity   int
}

func (ButterflyValve) choices(field string) []string {
	return map[string][]string{
		"Size":       pipeChoices["Size"],
		"Connection": []string{"threaded", "grooved", "flanged"},
	}[field]
}

type BallValve struct {
	Size       string
	Connection string
	Quantity   int
}

func (BallValve) choices(field string) []string {
	return map[string][]string{
		"Size":       pipeChoices["Size"],
		"Connection": []string{"threaded", "grooved", "flanged"},
	}[field]
}

type InlineCheckValve struct {
	Size       string
	Connection string
	Quantity   int
}

func (InlineCheckValve) choices(field string) []string {
	return map[string][]string{
		"Size":       pipeChoices["Size"],
		"Connection": []string{"grooved", "flanged"},
	}[field]
}

type SwingCheckValve struct {
	Size       string
	Connection string
	Quantity   int
}

func (SwingCheckValve) choices(field string) []string {
	return map[string][]string{
		"Size":       pipeChoices["Size"],
		"Connection": []string{"threaded", "grooved", "flanged"},
	}[field]
}

type RiserManifold struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (RiserManifold) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
	}[field]
}

type TestDrainAssembly struct {
	Size        string
	OrificeSize string `optional:"true"`
	Quantity    int
}

func (TestDrainAssembly) choices(field string) []string {
	return map[string][]string{
		"Size":        pipeChoices["Size"],
		"OrificeSize": []string{"½″", "¾″"},
	}[field]
}

type DelugeValve struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (DelugeValve) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
	}[field]
}

type PreActionValve struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PreActionValve) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
	}[field]
}

type DryPipeValve struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (DryPipeValve) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
	}[field]
}

type PressureReducingValve struct {
	Size     string
	PSI      int    `optional:"true"`
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PressureReducingValve) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
	}[field]
}

type BandHanger struct {
	Size     string
	Quantity int
}

func (BandHanger) choices(string) []string { return pipeChoices["Size"] }

type ClevisHanger struct {
	Size     string
	Quantity int
}

func (ClevisHanger) choices(string) []string { return pipeChoices["Size"] }

type LoopHanger struct {
	Size     string
	Quantity int
}

func (LoopHanger) choices(string) []string { return pipeChoices["Size"] }

type SurgeRestraint struct {
	Size     string
	Quantity int
}

func (SurgeRestraint) choices(string) []string { return pipeChoices["Size"] }

type BeamClamp struct {
	Size     string
	Quantity int
}

func (BeamClamp) choices(string) []string { return hardwareSizes }

type ConcreteInsert struct {
	Size     string
	Quantity int
}

func (ConcreteInsert) choices(string) []string { return hardwareSizes }

type CeilingFlange struct {
	Size     string
	Quantity int
}

func (CeilingFlange) choices(string) []string { return hardwareSizes }

type SideBeamBracket struct {
	Size     string
	Quantity int
}

func (SideBeamBracket) choices(string) []string { return hardwareSizes }

type TOLCO900Attachment struct {
	Size     string
	Quantity int
}

func (TOLCO900Attachment) choices(string) []string { return hardwareSizes }

type TransitionalFitting struct {
	Size     string
	Quantity int
}

func (TransitionalFitting) choices(string) []string { return pipeChoices["Size"] }

type SplayRestraintKit struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (SplayRestraintKit) choices(string) []string { return nil }

type ThreadedRod struct {
	Size     string
	Length   string
	Quantity int
}

func (ThreadedRod) choices(field string) []string {
	return map[string][]string{
		"Size":   hardwareSizes,
		"Length": rodLengths,
	}[field]
}

type ConcreteAnchor struct {
	Size     string
	Length   string
	Type     string
	Quantity int
}

func (ConcreteAnchor) choices(field string) []string {
	return map[string][]string{
		"Size":   hardwareSizes,
		"Length": fastenerLengths,
		"Type":   []string{"wedge", "sleeve"},
	}[field]
}

type HexBolt struct {
	Size     string
	Length   string
	Quantity int
}

func (HexBolt) choices(field string) []string {
	return map[string][]string{
		"Size":   hardwareSizes,
		"Length": fastenerLengths,
	}[field]
}

type Nut struct {
	Size     string
	Quantity int
}

func (Nut) choices(string) []string { return hardwareSizes }

type Washer struct {
	Size     string
	Quantity int
}

func (Washer) choices(string) []string { return hardwareSizes }

type SiameseConnection struct {
	Size     string
	Type     string
	Quantity int
}

func (SiameseConnection) choices(field string) []string {
	return map[string][]string{
		"Size": []string{"4″", "6″"},
		"Type": []string{"straight", "90° angle"},
	}[field]
}

type FDCWallEscutcheon struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

// https://www.potterroemer.com/products/fire-department-connections/wall-plates-fire-dept-connection-5600
func (FDCWallEscutcheon) choices(field string) []string {
	if field == "Size" {
		return []string{"9″ × 15″", "9″ × 20″", "9″ × 30″", "16″ × 16″", "9″ × 44″"}
	}
	return nil
}

type FDCPlug struct {
	Size     string
	Material string
	Quantity int
}

func (FDCPlug) choices(field string) []string {
	return map[string][]string{
		"Size":     []string{"2½″"},
		"Material": []string{"brass", "plastic"},
	}[field]
}

type FDCCap struct {
	Size     string
	Material string
	Quantity int
}

func (FDCCap) choices(field string) []string {
	return map[string][]string{
		"Size":     []string{"2½″"},
		"Material": []string{"brass", "plastic"},
	}[field]
}

type BreakAwayCap struct {
	Size     string
	Quantity int
}

func (BreakAwayCap) choices(string) []string { return []string{"2½″"} }

type Snoot struct {
	Size     string
	Quantity int
}

func (Snoot) choices(string) []string { return []string{"2½″"} }

type IdentificationSign struct {
	Text     string
	Quantity int
}

func (IdentificationSign) choices(string) []string {
	return []string{
		"Alarm Test",
		"Automatic Sprinkler",
		"Auxiliary Drain",
		"Control Valve",
		"Deluge Valve",
		"Do Not Close This Valve",
		"Dry Pipe Valve",
		"FDC",
		"Fire Pump",
		"Fire Pump Room",
		"Fire Pump Test Header",
		"Floor Control Valve",
		"Inspector's Test",
		"Keep Valve Open",
		"Low Point Drain",
		"Main Control Valve",
		"Main Drain",
		"Normally Closed",
		"Normally Open",
		"Preaction Valve",
		"Sectional Control Valve",
		"Spare Sprinklers",
		"Sprinkler & Standpipe",
		"Sprinkler Riser",
		"Sprinkler Riser Room",
		"Standpipe",
		"Test & Drain",
	}
}

type CustomIdentificationSign struct {
	Text     string `input:"text"`
	Quantity int
}

func (CustomIdentificationSign) choices(string) []string { return nil }

type HydraulicPlacard struct {
	Text     string `input:"text"`
	Quantity int
}

func (HydraulicPlacard) choices(string) []string { return nil }

type WaterflowDetector struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (WaterflowDetector) choices(field string) []string {
	return map[string][]string{
		"Size": pipeChoices["Size"],
	}[field]
}

type PressureSwitch struct {
	Type     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PressureSwitch) choices(field string) []string {
	return map[string][]string{
		"Type": []string{"high air", "low air"},
	}[field]
}

type TamperSwitch struct {
	ValveType string
	Model     string `input:"text" optional:"true"`
	Quantity  int
}

func (TamperSwitch) choices(field string) []string {
	return map[string][]string{
		"ValveType": []string{"butterfly", "OS&Y"},
	}[field]
}

type WaterMotorGong struct {
	Size     string
	Quantity int
}

func (WaterMotorGong) choices(string) []string { return pipeChoices["Size"] }

type ElectricBell struct {
	Voltage  string `optional:"true"`
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (ElectricBell) choices(field string) []string {
	return map[string][]string{
		"Voltage": []string{"12 VDC", "24 VDC", "120 VAC"},
	}[field]
}

type HornStrobe struct {
	Voltage  string `optional:"true"`
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HornStrobe) choices(field string) []string {
	return map[string][]string{
		"Voltage": []string{"12 VDC", "24 VDC", "120 VAC"},
	}[field]
}

type SpareHeadCabinet struct {
	Capacity string
	Quantity int
}

func (SpareHeadCabinet) choices(string) []string { return []string{"6 heads", "12 heads", "24 heads"} }

type PipeCutter struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PipeCutter) choices(string) []string { return nil }

type GroovingMachine struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (GroovingMachine) choices(string) []string { return nil }

type Threader struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (Threader) choices(string) []string { return nil }

type CenterPunchTool struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (CenterPunchTool) choices(string) []string { return nil }

type PipeWrench struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PipeWrench) choices(string) []string { return nil }

type AdjustableWrench struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (AdjustableWrench) choices(string) []string { return nil }

type SocketSet struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (SocketSet) choices(string) []string { return nil }

type TapeMeasure struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (TapeMeasure) choices(string) []string { return nil }

type ImpactDriver struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (ImpactDriver) choices(string) []string { return nil }

type HammerDrill struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HammerDrill) choices(string) []string { return nil }

type HoleSaw struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HoleSaw) choices(string) []string { return nil }

type PipeReamer struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PipeReamer) choices(string) []string { return nil }

type ChainVise struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (ChainVise) choices(string) []string { return nil }

type ReciprocatingSaw struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (ReciprocatingSaw) choices(string) []string { return nil }

type PitotTube struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (PitotTube) choices(string) []string { return nil }

type HydrostaticPump struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HydrostaticPump) choices(string) []string { return nil }

type Level struct {
	Type     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (Level) choices(field string) []string {
	if field == "Type" {
		return []string{"torpedo", "box", "laser"}
	}
	return nil
}

type TeflonTape struct {
	Size     string
	Quantity int
}

func (TeflonTape) choices(string) []string { return []string{"½″", "¾″", "1″"} }

type PipeJointCompound struct {
	Size     string
	Quantity int
}

func (PipeJointCompound) choices(string) []string { return containerSizes }

type CuttingOil struct {
	Size     string
	Quantity int
}

func (CuttingOil) choices(string) []string { return containerSizes }

type FireStopCaulk struct {
	Size     string
	Quantity int
}

func (FireStopCaulk) choices(string) []string { return []string{"10.1 oz", "20 oz", "28 oz"} }

type HardHat struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HardHat) choices(string) []string { return nil }

type SafetyGlasses struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (SafetyGlasses) choices(string) []string { return nil }

type WorkGloves struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (WorkGloves) choices(field string) []string {
	if field == "Size" {
		return []string{"S", "M", "L", "XL", "2XL"}
	}
	return nil
}

type HearingProtection struct {
	Type     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (HearingProtection) choices(field string) []string {
	if field == "Type" {
		return []string{"earplugs", "earmuffs"}
	}
	return nil
}

type SafetyVest struct {
	Size     string
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (SafetyVest) choices(field string) []string {
	if field == "Size" {
		return []string{"S", "M", "L", "XL", "2XL", "3XL", "4XL", "5XL"}
	}
	return nil
}

type FaceShield struct {
	Model    string `input:"text" optional:"true"`
	Quantity int
}

func (FaceShield) choices(string) []string { return nil }

type DrinkingWater struct {
	Quantity int
}

func (DrinkingWater) choices(string) []string { return nil }

type RoughInspectionRequest struct {
	Quantity int
}

func (RoughInspectionRequest) choices(string) []string { return nil }

type FinalInspectionRequest struct {
	Quantity int
}

func (FinalInspectionRequest) choices(string) []string { return nil }

type partLink struct {
	Name, URL string
	Form      http.HandlerFunc
}

type partCategory struct {
	ID, Name string
	Parts    []partLink
}

var partCategories = []partCategory{
	{"pipe-fittings", "Pipe & Fittings", []partLink{
		{"nipple", "/nipple", partForm[Nipple]},
		{"pipe", "/pipe", partForm[Pipe]},
		{"mechanical tee", "/mechanical-tee", partForm[MechanicalTee]},
		{"screwed tee", "/screwed-tee", partForm[ScrewedTee]},
		{"grooved elbow", "/parts/grooved-elbow", partForm[GroovedElbow]},
		{"grooved tee", "/parts/grooved-tee", partForm[GroovedTee]},
		{"grooved cross", "/parts/grooved-cross", partForm[GroovedCross]},
		{"grooved cap", "/parts/grooved-cap", partForm[GroovedCap]},
		{"add-a-cap", "/parts/add-a-cap", partForm[AddACap]},
		{"tap-cap", "/parts/tap-cap", partForm[TapCap]},
		{"grooved reducer", "/parts/grooved-reducer", partForm[GroovedReducer]},
		{"concentric reducer", "/parts/concentric-reducer", partForm[ConcentricReducer]},
		{"threaded elbow", "/parts/threaded-elbow", partForm[ThreadedElbow]},
		{"threaded coupling", "/parts/threaded-coupling", partForm[ThreadedCoupling]},
		{"threaded union", "/parts/threaded-union", partForm[ThreadedUnion]},
		{"threaded bushing", "/parts/threaded-bushing", partForm[ThreadedBushing]},
		{"CPVC pipe", "/parts/cpvc-pipe", partForm[CPVCPipe]},
		{"CPVC adapter", "/parts/cpvc-adapter", partForm[CPVCAdapter]},
		{"CPVC tee", "/parts/cpvc-tee", partForm[CPVCTee]},
		{"CPVC elbow", "/parts/cpvc-elbow", partForm[CPVCElbow]},
		{"CPVC solvent cement", "/parts/cpvc-cement", partForm[CPVCCement]},
		{"one part glue", "/parts/one-part-glue", partForm[OnePartGlue]},
		{"seamless steel pipe", "/parts/seamless-steel-pipe", partForm[SeamlessSteelPipe]},
		{"copper tubing", "/parts/copper-tubing", partForm[CopperTubing]},
		{"copper fitting", "/parts/copper-fitting", partForm[CopperFitting]},
	}},
	{"couplings-flange-adapters", "Couplings & Flange Adapters", []partLink{
		{"grooved coupling", "/grooved-coupling", partForm[GroovedCoupling]},
		{"grooved reducing coupling", "/grooved-reducing-coupling", partForm[GroovedReducingCoupling]},
		{"grooved flange adapter", "/grooved-flange-adapter", partForm[GroovedFlangeAdapter]},
		{"gasket", "/parts/gasket", partForm[Gasket]},
		{"pipe lubricant", "/parts/pipe-lubricant", partForm[PipeLubricant]},
	}},
	{"sprinkler-heads", "Fire Sprinkler Heads", []partLink{
		{"commercial sprinkler", "/parts/commercial-sprinkler", partForm[CommercialSprinkler]},
		{"residential sprinkler", "/parts/residential-sprinkler", partForm[ResidentialSprinkler]},
		{"ESFR sprinkler", "/parts/esfr-sprinkler", partForm[ESFRSprinkler]},
		{"CMSA sprinkler", "/parts/cmsa-sprinkler", partForm[CMSASprinkler]},
		{"dry pendent", "/parts/dry-pendent", partForm[DryPendent]},
		{"escutcheon", "/parts/escutcheon", partForm[Escutcheon]},
		{"head guard", "/parts/head-guard", partForm[HeadGuard]},
		{"baffle", "/parts/baffle", partForm[Baffle]},
	}},
	{"valves-control-devices", "Valves & Control Devices", []partLink{
		{"OS&Y gate valve", "/parts/osy-gate-valve", partForm[OSYGateValve]},
		{"butterfly valve with tamper switch", "/parts/butterfly-valve", partForm[ButterflyValve]},
		{"ball valve", "/parts/ball-valve", partForm[BallValve]},
		{"inline check valve", "/parts/inline-check-valve", partForm[InlineCheckValve]},
		{"swing check valve", "/parts/swing-check-valve", partForm[SwingCheckValve]},
		{"residential riser manifold", "/parts/riser-manifold", partForm[RiserManifold]},
		{"test and drain assembly", "/parts/test-drain-assembly", partForm[TestDrainAssembly]},
		{"deluge valve", "/parts/deluge-valve", partForm[DelugeValve]},
		{"pre-action valve", "/parts/pre-action-valve", partForm[PreActionValve]},
		{"dry pipe valve", "/parts/dry-pipe-valve", partForm[DryPipeValve]},
		{"pressure reducing valve", "/parts/pressure-reducing-valve", partForm[PressureReducingValve]},
	}},
	{"hangers-supports-fasteners", "Hangers, Supports & Fasteners", []partLink{
		{"pipe stand", "/pipe-stand", partForm[PipeStand]},
		{"band hanger", "/parts/band-hanger", partForm[BandHanger]},
		{"clevis hanger", "/parts/clevis-hanger", partForm[ClevisHanger]},
		{"loop hanger", "/parts/loop-hanger", partForm[LoopHanger]},
		{"surge restraint", "/parts/surge-restraint", partForm[SurgeRestraint]},
		{"beam clamp", "/parts/beam-clamp", partForm[BeamClamp]},
		{"concrete insert", "/parts/concrete-insert", partForm[ConcreteInsert]},
		{"ceiling flange", "/parts/ceiling-flange", partForm[CeilingFlange]},
		{"side-beam bracket", "/parts/side-beam-bracket", partForm[SideBeamBracket]},
		{"Eaton TOLCO Fig. 900 structural attachment", "/parts/tolco-900-attachment", partForm[TOLCO900Attachment]},
		{"transitional fitting", "/parts/transitional-fitting", partForm[TransitionalFitting]},
		{"splay restraint wire kit", "/parts/splay-restraint-kit", partForm[SplayRestraintKit]},
		{"threaded rod", "/parts/threaded-rod", partForm[ThreadedRod]},
		{"concrete anchor", "/parts/concrete-anchor", partForm[ConcreteAnchor]},
		{"hex bolt", "/parts/hex-bolt", partForm[HexBolt]},
		{"nut", "/parts/nut", partForm[Nut]},
		{"washer", "/parts/washer", partForm[Washer]},
	}},
	{"fdc-access", "Fire Department Connections (FDC) & Access", []partLink{
		{"Siamese connection", "/parts/siamese-connection", partForm[SiameseConnection]},
		{"flush-mount FDC wall escutcheon", "/parts/fdc-wall-escutcheon", partForm[FDCWallEscutcheon]},
		{"FDC plug with chain", "/parts/fdc-plug", partForm[FDCPlug]},
		{"FDC cap with chain", "/parts/fdc-cap", partForm[FDCCap]},
		{"break-away cap", "/parts/break-away-cap", partForm[BreakAwayCap]},
		{"snoot", "/parts/snoot", partForm[Snoot]},
		{"identification sign", "/parts/identification-sign", partForm[IdentificationSign]},
		{"custom identification sign", "/parts/custom-identification-sign", partForm[CustomIdentificationSign]},
		{"custom hydraulic placard", "/parts/hydraulic-placard", partForm[HydraulicPlacard]},
	}},
	{"alarm-monitoring-electrical", "Alarm, Monitoring & Electrical", []partLink{
		{"vane-type waterflow detector", "/parts/waterflow-detector", partForm[WaterflowDetector]},
		{"supervisory pressure switch", "/parts/pressure-switch", partForm[PressureSwitch]},
		{"tamper switch", "/parts/tamper-switch", partForm[TamperSwitch]},
		{"water motor gong", "/parts/water-motor-gong", partForm[WaterMotorGong]},
		{"electric bell", "/parts/electric-bell", partForm[ElectricBell]},
		{"horn/strobe", "/parts/horn-strobe", partForm[HornStrobe]},
	}},
	{"cabinets-tools-consumables", "Cabinets & Consumables", []partLink{
		{"spare head cabinet", "/parts/spare-head-cabinet", partForm[SpareHeadCabinet]},
		{"Teflon tape", "/parts/teflon-tape", partForm[TeflonTape]},
		{"pipe joint compound", "/parts/pipe-joint-compound", partForm[PipeJointCompound]},
		{"cutting oil", "/parts/cutting-oil", partForm[CuttingOil]},
		{"fire-stop caulking", "/parts/fire-stop-caulk", partForm[FireStopCaulk]},
	}},
	{"tools-of-the-trade", "Tools of the Trade", []partLink{
		{"hydrostatic pump", "/parts/hydrostatic-pump", partForm[HydrostaticPump]},
		{"pitot tube", "/parts/pitot-tube", partForm[PitotTube]},
		{"level", "/parts/level", partForm[Level]},
		{"sprinkler wrench", "/parts/sprinkler-wrench", partForm[SprinklerWrench]},
		{"pipe cutter", "/parts/pipe-cutter", partForm[PipeCutter]},
		{"grooving machine", "/parts/grooving-machine", partForm[GroovingMachine]},
		{"threader", "/parts/threader", partForm[Threader]},
		{"center-punch tool", "/parts/center-punch-tool", partForm[CenterPunchTool]},
		{"pipe wrench", "/parts/pipe-wrench", partForm[PipeWrench]},
		{"adjustable wrench", "/parts/adjustable-wrench", partForm[AdjustableWrench]},
		{"socket set", "/parts/socket-set", partForm[SocketSet]},
		{"tape measure", "/parts/tape-measure", partForm[TapeMeasure]},
		{"impact driver", "/parts/impact-driver", partForm[ImpactDriver]},
		{"hammer drill", "/parts/hammer-drill", partForm[HammerDrill]},
		{"hole saw", "/parts/hole-saw", partForm[HoleSaw]},
		{"pipe reamer", "/parts/pipe-reamer", partForm[PipeReamer]},
		{"chain vise", "/parts/chain-vise", partForm[ChainVise]},
		{"reciprocating saw", "/parts/reciprocating-saw", partForm[ReciprocatingSaw]},
	}},
	{"ppe", "PPE", []partLink{
		{"hard hat", "/parts/hard-hat", partForm[HardHat]},
		{"safety glasses", "/parts/safety-glasses", partForm[SafetyGlasses]},
		{"work gloves (pairs)", "/parts/work-gloves", partForm[WorkGloves]},
		{"hearing protection (pairs)", "/parts/hearing-protection", partForm[HearingProtection]},
		{"safety vest", "/parts/safety-vest", partForm[SafetyVest]},
		{"face shield", "/parts/face-shield", partForm[FaceShield]},
		{"drinking water", "/parts/drinking-water", partForm[DrinkingWater]},
	}},
	{"rough-final-inspection", "Rough & Final Inspection", []partLink{
		{"rough inspection request", "/inspections/rough", partForm[RoughInspectionRequest]},
		{"final inspection request", "/inspections/final", partForm[FinalInspectionRequest]},
	}},
}

func findCategory(id string) *partCategory {
	for i := range partCategories {
		if partCategories[i].ID == id {
			return &partCategories[i]
		}
	}
	return nil
}

type formField struct {
	Name, Value string
	Label       string
	Choices     []string
	Error       error
	Text        bool
	Optional    bool
}

type partFormData struct {
	PartName string
	Target   string
	Fields   []formField
	Initial  bool
}

const pageStart = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pot part ordering tool</title>
<style>
html { font-size: 150%; }
body { margin: 0; padding: 0.2rem; font-family: sans-serif; line-height: 1.4; overflow-wrap: anywhere; }
*, *::before, *::after { box-sizing: border-box; }
h1 { font-size: 1.25rem; margin: 0 0 0.4rem; }
h2 { font-size: 1.1rem; }
section { margin-bottom: 1rem; }
form { margin-bottom: 0.4rem; }
form > div { margin-bottom: 0.3rem; }
label { display: block; }
input, select, button { font: inherit; max-width: 100%; min-width: 44px; min-height: 44px; padding: 0.1em; }
input, select { display: block; width: 100%; }
button { white-space: normal; overflow-wrap: anywhere; }
a { display: inline-block; min-width: 44px; min-height: 44px; padding-block: 0.1em; }
</style>
</head>
<body>
`

const pageEnd = `</body></html>`

var indexTemplate = template.Must(template.New("index").Funcs(template.FuncMap{
	"formatPart": formatPart,
}).Parse(pageStart + `
{{if .Selected}}
<nav aria-label="Navigation"><a href="/">All categories</a> · <a href="#cart">Cart</a></nav>
<h1>{{.Selected.Name}}</h1>
<nav aria-label="Parts">
{{range .Selected.Parts}}<div><a href="{{.URL}}">{{.Name}}</a></div>{{else}}<p>No parts added yet.</p>{{end}}
</nav>
{{else}}
<h1>Part categories</h1>
<nav aria-label="Categories">
{{range .Categories}}<div><a href="/?category={{.ID}}">{{.Name}}</a></div>{{end}}
</nav>
{{end}}
<section id="cart">
<h2>Cart</h2>
{{range $index, $item := .Items}}
<form action="/remove" method="post"><p>{{formatPart $item}}
  <button type="submit" name="index" value="{{$index}}">delete</button>
</p></form>
{{else}}<p>Your cart is empty.</p>{{end}}
{{if .Items}}<form action="/submit" method="post"><button type="submit">submit order</button></form>{{end}}
</section>
` + pageEnd))

var partTemplate = template.Must(template.New("part").Parse(pageStart + `
<nav aria-label="Navigation"><a href="/">Categories &amp; cart</a></nav>
<h1>{{.PartName}}</h1>
<form action="{{.Target}}" method="post">
{{range .Fields}}
<div>
  <label for="{{.Name}}">{{.Label}}{{if .Optional}} (optional){{end}}</label>
  {{if .Choices}}
  <select id="{{.Name}}" name="{{.Name}}"{{if not .Optional}} required{{end}}>
    {{if .Optional}}<option value=""></option>{{end}}
    {{$value := .Value}}{{range .Choices}}
    <option value="{{.}}" {{if eq $value .}}selected{{end}}>{{.}}</option>
    {{end}}
  </select>
  {{else if .Text}}
  <input type="text" id="{{.Name}}" name="{{.Name}}" value="{{.Value}}"{{if not .Optional}} required{{end}}>
  {{else}}
  <input type="number" id="{{.Name}}" name="{{.Name}}" value="{{.Value}}" min="1"{{if not .Optional}} required{{end}}>
  {{end}}
  {{with .Error}}<div role="alert">{{.}}</div>{{end}}
</div>
{{end}}
<button type="submit">save part</button>
</form>
{{if .Initial}}
<script>
document.querySelectorAll('select').forEach(select => select.selectedIndex = -1);
</script>
{{end}}
` + pageEnd))

func partForm[T part](w http.ResponseWriter, r *http.Request) {
	var p T
	var errs map[string]error
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		errs = mapForm(r, &p)
		if len(errs) == 0 {
			log.Printf("post %s", formatPart(p))
			id := sessionID(w, r)
			cartsMu.Lock()
			carts[id] = append(carts[id], p)
			cartsMu.Unlock()
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	var fields []formField
	value := reflect.ValueOf(p)
	for index := range value.NumField() {
		field := value.Type().Field(index)
		input := formField{
			Name:     field.Name,
			Label:    field.Tag.Get("label"),
			Value:    fmt.Sprint(value.Field(index).Interface()),
			Error:    errs[field.Name],
			Text:     field.Tag.Get("input") == "text",
			Optional: field.Tag.Get("optional") == "true",
		}
		if input.Label == "" {
			input.Label = field.Name
		}
		if input.Optional && value.Field(index).IsZero() {
			input.Value = ""
		}
		if r.Method == http.MethodPost {
			input.Value = r.Form.Get(field.Name)
		}
		if value.Field(index).Kind() == reflect.String {
			input.Choices = p.choices(field.Name)
		}
		fields = append(fields, input)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(errs) > 0 {
		w.WriteHeader(http.StatusBadRequest)
	}

	if err := partTemplate.Execute(w, partFormData{
		PartName: value.Type().Name(),
		Target:   r.URL.Path, Fields: fields,
		Initial: r.Method == http.MethodGet,
	}); err != nil {
		log.Println(err)
	}
}

func readUsers(r io.Reader) (map[string]string, error) {
	users := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		username, password, found := strings.Cut(scanner.Text(), "|")
		if !found {
			return nil, fmt.Errorf("passwd:%d: expected username|password", line)
		}
		users[username] = password
	}
	return users, scanner.Err()
}
func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var username string
		loggedIn := false
		if cookie, err := r.Cookie("sid"); err == nil {
			sessionsMu.Lock()
			username, loggedIn = sessions[cookie.Value]
			sessionsMu.Unlock()
		}

		log.Printf("%s %s %s user=%q", r.Method, r.URL.Path, r.RemoteAddr, username)
		if !loggedIn {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
func login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Println(r.URL.Path, r.Method, r.RemoteAddr)
	io.WriteString(w, pageStart+`<form method="post" action="/login">
	<input name="username" placeholder="username" required autofocus><br>
        <input name="password" type="password" placeholder="password" required><br>
        <button type="submit">login</button>
    </form>`+pageEnd)
}

func authenticate(users map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path, r.Method, r.RemoteAddr)
		username := r.FormValue("username")
		if password, ok := users[username]; !ok || password != r.FormValue("password") {
			http.Error(w, "invalid login", http.StatusUnauthorized)
			return
		}

		id := sessionID(w, r)

		sessionsMu.Lock()
		sessions[id] = username
		sessionsMu.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	var selected *partCategory
	if categoryID := r.URL.Query().Get("category"); categoryID != "" {
		selected = findCategory(categoryID)
		if selected == nil {
			http.NotFound(w, r)
			return
		}
	}
	id := sessionID(w, r)

	cartsMu.Lock()
	items := slices.Clone(carts[id])
	cartsMu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := indexTemplate.Execute(w, struct {
		Categories []partCategory
		Selected   *partCategory
		Items      []part
	}{partCategories, selected, items}); err != nil {
		log.Println(err)
	}
}

func removePart(w http.ResponseWriter, r *http.Request) {
	id := sessionID(w, r)
	index, err := strconv.Atoi(r.FormValue("index"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cartsMu.Lock()
	cart := carts[id]
	if index < 0 || index >= len(cart) {
		cartsMu.Unlock()
		http.NotFound(w, r)
		return
	}
	carts[id] = slices.Delete(cart, index, index+1)
	cartsMu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func sendOrder(o order) error {
	return smtp.SendMail(
		smtpServer+":"+smtpPort,
		smtp.PlainAuth("", senderMail, senderPass, smtpServer),
		senderMail,
		to,
		[]byte(fmt.Sprintf("subject: order alert\n\n%s\nrequester: %s", o, o.username)),
	)
}

type order struct {
	id, username string
	items        []part
}

func (o order) String() string {
	var text strings.Builder
	for _, item := range o.items {
		text.WriteString(formatPart(item))
		text.WriteByte('\n')
	}
	return text.String()
}

func submit(recordOrder func(string, ...any) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := sessionID(w, r)
		cartsMu.Lock()
		items := carts[id]
		delete(carts, id)
		cartsMu.Unlock()
		if len(items) == 0 {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		sessionsMu.Lock()
		username := sessions[id]
		sessionsMu.Unlock()
		o := order{id: rand.Text(), username: username, items: items}

		if err := recordOrder(
			"%s: submitted by %q: %q",
			o.id,
			o.username,
			strings.TrimSuffix(o.String(), "\n"),
		); err != nil {
			log.Printf("ERROR %s: %q", o.id, err.Error())
		}

		err := sendOrder(o)
		status := "sent"
		if err != nil {
			log.Printf("ERROR %s: %q", o.id, err.Error())
			status = fmt.Sprintf("failed: %v", err)
		}
		if logErr := recordOrder("%s: email %q", o.id, status); logErr != nil {
			log.Printf("ERROR %s: email %q: %q", o.id, status, logErr.Error())
		}
		if err != nil {
			cartsMu.Lock()
			carts[id] = append(o.items, carts[id]...)
			cartsMu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, pageStart+`
			    <div>
				<h1>sent</h1>
				<a href="/">&larr; back</a>
			    </div>
			`+pageEnd)
	}
}

func sessionID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie("sid"); err == nil {
		return c.Value
	}

	id := rand.Text()
	http.SetCookie(w, &http.Cookie{
		Name: "sid", Value: id, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	return id
}

func registerPartForms(mux *http.ServeMux) {
	for _, category := range partCategories {
		for _, item := range category.Parts {
			mux.HandleFunc("GET "+item.URL, requireLogin(item.Form))
			mux.HandleFunc("POST "+item.URL, requireLogin(item.Form))
		}
	}
}

func main() {
	ordersLog := log.New(os.Stderr, "", log.LstdFlags)
	ordersFile, err := os.OpenFile("orders.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Printf("ERROR %q", err.Error())
		ordersLog.SetPrefix(fmt.Sprintf("ERROR %q: ", err.Error()))
	} else {
		defer ordersFile.Close()
		ordersLog.SetOutput(ordersFile)
	}

	recordOrder := func(format string, args ...any) error {
		if err := ordersLog.Output(2, fmt.Sprintf(format, args...)); err != nil {
			return err
		}
		if ordersFile != nil {
			return ordersFile.Sync()
		}
		return nil
	}

	f, err := os.Open("passwd")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	users, err := readUsers(f)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("GET /login", login)

	http.HandleFunc("POST /login", authenticate(users))
	registerPartForms(http.DefaultServeMux)

	http.HandleFunc("GET /{$}", requireLogin(index))
	http.HandleFunc("POST /remove", requireLogin(removePart))
	http.HandleFunc("POST /submit", requireLogin(submit(recordOrder)))
	go func() {
		redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r,
				"https://jjdev.com"+r.URL.RequestURI(),
				http.StatusPermanentRedirect)
		})
		log.Fatal(http.ListenAndServe(":8080", redirect))
	}()

	log.Fatal(http.ListenAndServeTLS(
		":8443",
		"/etc/letsencrypt/live/jjdev.com/fullchain.pem",
		"/etc/letsencrypt/live/jjdev.com/privkey.pem",
		nil,
	))
}
