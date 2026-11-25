package main

import (
	"github.com/ccc112233/dag/internal/plugin"
	northamqp "github.com/ccc112233/dag/plugins/north/amqp"
	"github.com/ccc112233/dag/plugins/north/httpchan"
	northmqtt "github.com/ccc112233/dag/plugins/north/mqtt"
	northstub "github.com/ccc112233/dag/plugins/north/stub"
	"github.com/ccc112233/dag/plugins/south/modbus"
	"github.com/ccc112233/dag/plugins/south/opcua"
	southstub "github.com/ccc112233/dag/plugins/south/stub"
)

// registerPlugins wires built-in and example plugins into the registry.
// New protocol plugins: implement the interface, then RegisterSouth/RegisterNorth here.
func registerPlugins(reg *plugin.Registry) {
	reg.RegisterSouth(southstub.Driver{})
	reg.RegisterSouth(southstub.FailDriver{})
	reg.RegisterSouth(modbus.Driver{})
	reg.RegisterSouth(opcua.Driver{})

	reg.RegisterNorth(northstub.Driver{})
	reg.RegisterNorth(httpchan.Driver{})
	reg.RegisterNorth(northmqtt.Driver{})
	reg.RegisterNorth(northamqp.Driver{})
}
