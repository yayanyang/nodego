package geoip

import(
	"testing"
	"net"
)

var database *GeoIPDB

func init(){
	print("~~~\n")
	database = Load("./GeoIPCountryWhois.csv")
}

func TestGeoIP(t *testing.T) {
	ip := net.ParseIP("103.14.100.100")

	t.Log(database.Search(ip))
	if database.Search(ip) != "HK" {
		t.Error("ip mismatch")
	}

	ip = net.ParseIP("171.216.90.18")
	t.Log(database.Search(ip))
	ip = net.ParseIP("8.35.201.33")
	t.Log(database.Search(ip))
	ip = net.ParseIP("62.216.125.241")
	t.Log(database.Search(ip))
}

func BenchmarkGeoIPSearch(b * testing.B) {
	ip := net.ParseIP("103.14.100.100")

	for i := 0; i < b.N; i++ {
		database.Search(ip)
	}
}