# prospect

## enumerations

## model

* Flight Model
* Engineering Model
* Training Model
* None

# product types

* High Rate Telemetry
* Medium Rate Telemetry
* High Rate Science Data
* High Rate Image Data
* Video Stream
* Command History
* Console Log
* Documentation
* Mission Data Base

### data source

* Science Run
* Engineering Test
* Troubleshooting
* Commissioning
* Experiment Sequence Test
* System Verification Test
* Baseline Data Collection
* Undefined

## plugins

### basic

basic plugin set the following experiment specific metadata:

* file.size

if no mime types are set in the module config or none match, the plugin set the mimetype
property to: **application/octet-stream**

if no type is set in the module config, the plugin set the type property to: **data**

### rt

rt plugin set the following experiment specific metadata:

* file.duration
* file.numrec
* file.size
* file.corrupted

if no mime types are set in the module config or none match, the plugin set the mimetype
property to: **application/octet-stream;access=sequential,form=unformatted**

if no type is set in the module config, the plugin set the type property to:
**medium rate telemetry**

### hadock

hadock plugin set the following experiment specific metadata:

* file.size
* hrd.channel
* hrd.source
* hrd.upi
* hrd.instance
* hrd.mode
* hrd.fcc
* hrd.pixels.x
* hrd.pixels.y
* hrd.invalid

if no mime types are set in the module config or none match, the plugin set the mimetype
property to: **application/octet-stream**

if no type is set in the module config, the plugin set the type property to:
**high rate data**

### icn - intre-console note

icn plugin set the following experiment specific metadata:

* file.size
* file.numrec
* ptr.%d.href
* ptr.%d.role

if no mime types are set in the module config or none match, the plugin set the mimetype
property to: **text/plain;access=sequential;form=block-formatted;type=icn**

if no type is set in the module config, the plugin set the type property to:
**intre-console note**

### icn - uplinked file

icn plugin set the following experiment specific metadata:

* file.size
* file.md5
* uplink.file.local
* uplink.file.uplink
* uplink.file.mmu
* uplink.time.uplink
* uplink.time.transfer
* uplink.source

if no mime types are set in the module config or none match, the plugin set the mimetype
property to: **text/plain**

if no type is set in the module config, the plugin set the type property to:
**uplink file**
