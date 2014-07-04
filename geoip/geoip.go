package geoip


import (
	"unsafe"
	"runtime"
	"net"
	"encoding/binary"
)

/*
#cgo CXXFLAGS: -std=c++11
#include <stdlib.h>

void* geoip_load(const char * csv);
void geoip_close(void* db);
const char* geoip_search(void* db, unsigned int ip);
*/
import "C"

type GeoIPDB struct {
	handle unsafe.Pointer
} 

func __close(db *GeoIPDB) {
	C.geoip_close(db.handle)
	runtime.SetFinalizer(db, nil)
}

func Load(path string) *GeoIPDB{
	csvPath := C.CString(path)
	
	defer func () {
		C.free(unsafe.Pointer(csvPath))
	}()

	database := &GeoIPDB{handle :C.geoip_load(csvPath)}

	runtime.SetFinalizer(database, __close)

	return database
}

func (database * GeoIPDB) Close() {
	__close(database)
}

func (database * GeoIPDB) Search(ip net.IP) string {
	ipv4 := ip.To4()

	if ipv4 != nil {
		target := C.uint(binary.BigEndian.Uint32(ipv4))
		ptr := C.geoip_search(database.handle,target)
		if unsafe.Pointer(ptr) != nil {
			return C.GoString(ptr)
		}
	}

	return ""
}