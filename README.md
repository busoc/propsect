# prospect

## configuration

prospect is configured with a configuration file (using the TOML format). This file
has the following tables (in TOML terminology) and options

### top level table

* archive    : directory where data files are metadata files (or zip file) will be created
* no-data    : tell prospect to only generate the metadata file
* directories: use the list of properties to generate the final directory tree structure where data files and metadata will be created

### meta

* acronym     : name of the experiment
* experiment  : full name of the experiment
* id          : erasmus experiment ID
* dtstart     : start date of the experiment
* dtend       : end date of the experiment
* fields      : list of research fields
* coordinators: list of people invovled in the experiments
* increments  : list of increments (start-end)

### meta.payload

* name   : full name of the payload
* acronym: acronym of the payload
* class  : class of the payload

### dataset
* rootdir  : not used
* owner    : dataset owner
* level    : processing level of the dataset
* integrity: hash algorithm to compute the digest of the data files
* model    : source having generating the dataset

### period
* dtstart: start date of a period of activity
* dtend  : end date of a period of activity
* source : activity performed during this period

### module
* module  : path to the plugin/module to be loaded by prospect
* location: path to data files (can be a pattern, a directory, a file - plugin specific)
* type    : product type handles by the plugin
* mime    : file format handles by the plugin

note: the type and mime option even if set, could be used or not by the plugin.

### module.mimetype
* extension: list of extensions (prefixed with a dot)
* mime     : mime type to be set for the given list of extension
* type     : product type matching the extension and the mime type


### sample configuration file (used for compgran)

```

archive = "var/sdc/fsl/compgran"
no-data = true
directories = [
  "model",
  "source",
  "mime",
  "year",
  "doy",
  "hour",
  "minute",
]

[meta]
acronym    = "compgran"
experiment = "Compaction and Sound in Granular Media"
id         = 9627 # ERASMUS EXPERIMENT ARCHIVE

dtstart    = 2018-07-19T16:36:00Z
dtend      = 2019-06-18T13:11:00Z

fields       = ["physical sciences", "granular system","soft matter"]
coordinators = []
increments = [
  "55-56",
  "57-58",
  "59-60",
]

  [[meta.payload]]
  name    = "Fluid Science Laboratory"
  acronym = "FSL"
  class   = 1

  [[meta.payload]]
  name    = "Soft Matter Dynamics"
  acronym = "SMD"
  class   = 2

[dataset]
rootdir   = "compgran"
owner     = "European Space Agency (ESA)"
level     = 0
integrity = "SHA-256"
model     = "flight model"

# [[period]]
# dtstart = 2018-07-19T16:36:00Z
# dtend   = 2019-06-18T13:11:00Z
# source  = "Science run"

[[module]]
module   = "lib/prospect/hadock.so"
location = "var/hadock/data/l0/OPS/images/**/*.*"
type     = "High Rate Images"

[[module]]
module   = "lib/prospect/hadock.so"
location = "var/hadock/data/l0/OPS/sciences/**/*.*"
type     = "High Rate Sciences"

[[module]]
module   = "lib/prospect/rt.so"
location = "var/cdmcs/fsl/hrdl/**/rt_??_??.dat"
type     = "High Rate Telemetry"

  [[module.mimetype]]
  extensions = [".dat", ".DAT"]
  mime       = "application/octet-stream;access=sequential,form=unformatted"
  type       = "raw telemetry"

[[module]]
module   = "lib/prospect/icn.so"
location = "tmp/uplinks.csv"
type     = ""
mime     = "text/plain"

  [[module.mimetype]]
  extensions = [".icn", ".ICN"]
  mime       = "text/plain;access=sequential;form=block-formatted;type=icn"
  type       = "Intre-console note"

  [[module.mimetype]]
  extensions = [".dat", ".DAT"]
  mime       = "text/plain"
  type       = "Uploaded file"

```

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
