package locations

import "sort"

type Location struct {
	Name        string  `json:"name"`
	CountryCode string  `json:"country_code"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Description string  `json:"description"`
	Region      string  `json:"region"`
}

var GlobalLocations = []Location{
	// North America
	{"New York", "US", 40.7128, -74.0060, "Financial Hub", "North America"},
	{"Los Angeles", "US", 34.0522, -118.2437, "Tech Hub", "North America"},
	{"Chicago", "US", 41.8781, -87.6298, "Industrial Hub", "North America"},
	{"Toronto", "CA", 43.6532, -79.3832, "Financial Center", "North America"},
	{"Vancouver", "CA", 49.2827, -123.1207, "Tech Hub", "North America"},
	
	// Europe
	{"London", "GB", 51.5074, -0.1278, "Financial Capital", "Europe"},
	{"Frankfurt", "DE", 50.1109, 8.6821, "Internet Exchange", "Europe"},
	{"Amsterdam", "NL", 52.3676, 4.9041, "Data Center Hub", "Europe"},
	{"Stockholm", "SE", 59.3293, 18.0686, "Nordic Tech Hub", "Europe"},
	{"Paris", "FR", 48.8566, 2.3522, "Cultural Center", "Europe"},
	
	// Asia-Pacific  
	{"Tokyo", "JP", 35.6762, 139.6503, "Tech Innovation", "Asia-Pacific"},
	{"Seoul", "KR", 37.5665, 126.9780, "Gaming Capital", "Asia-Pacific"},
	{"Singapore", "SG", 1.3521, 103.8198, "SE Asia Hub", "Asia-Pacific"},
	{"Hong Kong", "HK", 22.3193, 114.1694, "Financial Hub", "Asia-Pacific"},
	{"Sydney", "AU", -33.8688, 151.2093, "Pacific Hub", "Asia-Pacific"},
	
	// Others
	{"Dubai", "AE", 25.2048, 55.2708, "Middle East Hub", "Middle East"},
	{"São Paulo", "BR", -23.5505, -46.6333, "Economic Hub", "South America"},
}

func GetByRegion() map[string][]Location {
	regions := make(map[string][]Location)
	for _, loc := range GlobalLocations {
		regions[loc.Region] = append(regions[loc.Region], loc)
	}
	return regions
}

func GetRegions() []string {
	seen := make(map[string]bool)
	var regions []string
	for _, loc := range GlobalLocations {
		if !seen[loc.Region] {
			regions = append(regions, loc.Region)
			seen[loc.Region] = true
		}
	}
	sort.Strings(regions)
	return regions
}

func FindByName(name string) *Location {
	for _, loc := range GlobalLocations {
		if loc.Name == name {
			return &loc
		}
	}
	return nil
}
