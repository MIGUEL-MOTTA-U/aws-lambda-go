package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type ListingID string

func (id *ListingID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*id = ListingID(s)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*id = ListingID(fmt.Sprintf("%.0f", n))
		return nil
	}
	return fmt.Errorf("listing_id must be a string or number")
}

func (id ListingID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(id))
}

type Coordinates struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Location struct {
	Country      string      `json:"country"`
	State        string      `json:"state"`
	City         string      `json:"city"`
	Neighborhood string      `json:"neighborhood"`
	Address      string      `json:"address"`
	Stratum      int         `json:"stratum"`
	Coordinates  Coordinates `json:"coordinates"`
}

type Pricing struct {
	SalePrice        float64 `json:"sale_price"`
	RentPrice        float64 `json:"rent_price"`
	AdminFee         float64 `json:"admin_fee"`
	Taxes            float64 `json:"taxes"`
	Currency         string  `json:"currency"`
	DisplayPriceText string  `json:"display_price_text"`
}

type Areas struct {
	LandAreaM2    float64 `json:"land_area_m2"`
	BuiltAreaM2   float64 `json:"built_area_m2"`
	PrivateAreaM2 float64 `json:"private_area_m2"`
	LotAreaM2     float64 `json:"lot_area_m2"`
	FrontM        float64 `json:"front_m"`
	BackM         float64 `json:"back_m"`
}

type Layout struct {
	Bedrooms      int `json:"bedrooms"`
	Bathrooms     int `json:"bathrooms"`
	HalfBathrooms int `json:"half_bathrooms"`
	ParkingSpaces int `json:"parking_spaces"`
	Floors        int `json:"floors"`
	UnitFloor     int `json:"unit_floor"`
}

type Structure struct {
	YearBuilt           int    `json:"year_built"`
	AgeYears            int    `json:"age_years"`
	ConstructionQuality string `json:"construction_quality"`
	ConservationStatus  string `json:"conservation_status"`
	TerrainType         string `json:"terrain_type"`
	StructureType       string `json:"structure_type"`
	BuiltLevels         int    `json:"built_levels"`
}

type Features struct {
	Indoor     []string `json:"indoor"`
	Outdoor    []string `json:"outdoor"`
	Commercial []string `json:"commercial"`
	Project    []string `json:"project"`
}

type Media struct {
	Photos            []string `json:"photos"`
	PhotoCount        int      `json:"photo_count"`
	HasMap            bool     `json:"has_map"`
	HasVideo          bool     `json:"has_video"`
	HasFloorplans     bool     `json:"has_floorplans"`
	HasVirtualTour360 bool     `json:"has_virtual_tour_360"`
}

type Commercial struct {
	AgentName    string `json:"agent_name"`
	OfficeName   string `json:"office_name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	WhatsappLink string `json:"whatsapp_link"`
	OfficeHours  string `json:"office_hours"`
}

type ListingMetadata struct {
	UpdatedAt      string   `json:"updated_at"`
	UpdatedAgeText string   `json:"updated_age_text"`
	Breadcrumbs    []string `json:"breadcrumbs"`
	SourceSystem   string   `json:"source_system"`
}

type Listing struct {
	ListingID         ListingID       `json:"listing_id" gorm:"primaryKey;column:listing_id"`
	Slug              string          `json:"slug" gorm:"column:slug"`
	URL               string          `json:"url" gorm:"column:url"`
	Language          string          `json:"language" gorm:"column:language"`
	Title             string          `json:"title" gorm:"column:title"`
	PropertyType      string          `json:"property_type" gorm:"column:property_type"`
	Subtype           string          `json:"subtype" gorm:"column:subtype"`
	OperationType     string          `json:"operation_type" gorm:"column:operation_type"`
	PublicationStatus string          `json:"publication_status" gorm:"column:publication_status"`
	Location          Location        `json:"location" gorm:"type:jsonb;column:location"`
	Pricing           Pricing         `json:"pricing" gorm:"type:jsonb;column:pricing"`
	Areas             Areas           `json:"areas" gorm:"type:jsonb;column:areas"`
	Layout            Layout          `json:"layout" gorm:"type:jsonb;column:layout"`
	Structure         Structure       `json:"structure" gorm:"type:jsonb;column:structure"`
	Features          Features        `json:"features" gorm:"type:jsonb;column:features"`
	Media             Media           `json:"media" gorm:"type:jsonb;column:media"`
	Commercial        Commercial      `json:"commercial" gorm:"type:jsonb;column:commercial"`
	Metadata          ListingMetadata `json:"metadata" gorm:"type:jsonb;column:metadata"`
}

// Implement driver.Valuer and sql.Scanner for jsonb columns

func (l Location) Value() (driver.Value, error) {
	return json.Marshal(l)
}

func (l *Location) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("location scan source is not []byte")
	}
	return json.Unmarshal(b, l)
}

func (p Pricing) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *Pricing) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("pricing scan source is not []byte")
	}
	return json.Unmarshal(b, p)
}

func (a Areas) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *Areas) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("areas scan source is not []byte")
	}
	return json.Unmarshal(b, a)
}

func (l Layout) Value() (driver.Value, error) {
	return json.Marshal(l)
}

func (l *Layout) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("layout scan source is not []byte")
	}
	return json.Unmarshal(b, l)
}

func (s Structure) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *Structure) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("structure scan source is not []byte")
	}
	return json.Unmarshal(b, s)
}

func (f Features) Value() (driver.Value, error) {
	return json.Marshal(f)
}

func (f *Features) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("features scan source is not []byte")
	}
	return json.Unmarshal(b, f)
}

func (m Media) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *Media) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("media scan source is not []byte")
	}
	return json.Unmarshal(b, m)
}

func (c Commercial) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *Commercial) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("commercial scan source is not []byte")
	}
	return json.Unmarshal(b, c)
}

func (lm ListingMetadata) Value() (driver.Value, error) {
	return json.Marshal(lm)
}

func (lm *ListingMetadata) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("metadata scan source is not []byte")
	}
	return json.Unmarshal(b, lm)
}
