# prospect

```
usage:

$ prospect [-s schedule] config.toml
```

## schedule

prospect use two configuration files. The main configuration will be described in the next section.

The schedule configuration file is used to give to prospect a list of periods
during which kind of activities have been performed and which source have been used.

This list of periods should be given in a CSV file. this file should have the following fields (in the given order):

* activity start time (RFC3339 format)
* activity end time (RFC3339 format)
* activity type
* activity comment - should be present even if empty

When a data file match one of the activities found in the schedule, prospect will use the values of a matching activity to add specific metadata to this file.

Moreover, all files that does not belong to any of the activities will be discarded and
won't be saved into the archive.

## configuration

the configuration file and its structure of prospect is described in the sections that
follow:

### top table

* archive: directory where data files are metadata files (or zip file) will be created
* no-data: tell prospect to only generate the metadata file
* path   : [path pattern](#Path generation) used to build the final path of the file in the archive

### meta

the meta table (and its sub tables) groups all properties that describe the experiment itself. The options are used to generate the MD_EXP_<experiment>.xml file. The only exception is the dtstart, dtend options that can be used in parallel with the schedule option of the command.

* acronym     : name of the experiment
* experiment  : full name of the experiment
* id          : erasmus experiment ID
* dtstart     : start date of the experiment
* dtend       : end date of the experiment
* fields      : list of research fields
* coordinators: list of people invovled in the experiments
* increments  : list of increments (start-end)

#### meta.payload

* name   : full name of the payload
* acronym: acronym of the payload
* class  : class of the payload

### dataset

* rootdir  : not used
* owner    : dataset owner
* level    : processing level of the dataset
* integrity: [hash algorithm](#Supported hash algorithms) to compute the digest of the data files
* model    : source having generating the dataset

### module

* module  : path to the plugin/module to be loaded by prospect
* location: path to data files (can be a pattern, a directory, a file - plugin specific)
* type    : product type handles by the plugin
* mime    : file format handles by the plugin
* path    : [path pattern](#Path generation) used to build the final path of the file in the archive
* level   : product level
* config  : plugin specific configuration file
* acqtime : algorithm to be used to compute the acquisition time of a data file

notes:

* the type and mime option even if set, can be ignore by the plugin implementation.
* the level option even if set, can be ignore by the plugin implementation.
* the config option even if set, can be ignore by the plugin implementation.
* the acqtime option even if set, can be ignore by the plugin implementation.

#### module.mimetype

* extension: list of extensions (prefixed with a dot)
* mime     : mime type to be set for the given list of extension
* type     : product type matching the extension and the mime type

## Enumerations

### Model

* Flight Model
* Engineering Model
* Training Model
* None

### Product types

* High Rate Telemetry
* Medium Rate Telemetry
* High Rate Science Data
* High Rate Image Data
* Video Stream
* Command History
* Console Log
* Documentation
* Mission Data Base

### Data source

* Science Run
* Engineering Test
* Troubleshooting
* Commissioning
* Experiment Sequence Test
* System Verification Test
* Baseline Data Collection
* Undefined

# Globbing

pattern can be passed to the location option in the module tables. This pattern
have the following synyax:

* ?: match a single character
* *: match zero or multiple characters
* []: match a character in the given set of characters
* **: match any levels of sub directories
* !(): negate a matching
* @(): match alternative
* ?(): match zero or one time the given pattern
* +(): match at least one time the given pattern
* *(): match zero or multiples time the given pattern

# Supported hash algorithms

prospect can generate the digest for the data files with the following well known
algorithm:

* MD5
* SHA-1
* SHA-256
* SHA-512

# Path generation

in order to control the final location of files in the archive, prospect uses a
parameterizable path pattern via the {} notation. The parameters that can be used
in this pattern will be used to create the final path.

the following properties can be used:

* source
* model
* mime
* format
* type
* year
* doy
* month
* day
* hour
* minute
* second
* timestamp

note that any "propreties" not recognized by prospect will be injected by prospect
as is in the final path.

## Plugins

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
* ptr.%d.href
* ptr.%d.role

if no mime types are set in the module config or none match, the plugin set the mimetype
property to: **text/plain**

if no type is set in the module config, the plugin set the type property to:
**uplink file**

the icn plugin expects having as input file (specified in the location option) a csv file
with the following fields (in the given order):

* source ICN
* uplinked file
* original filename
* command filename
* filename used for uplink
* sid
* uplink time
* transfer time
* warning
* file size
* md5

this kind of file can be generated thanks to the script scripts/icn.awk

### csv

the csv plugin set the following experiment specific metadata:

* file.size
* file.duration
* file.numrec
* csv.%d.header

the csv plugin expects that all the rows in the input files contains exactly the same
number of fields.

### mbox

currently the mbox plugin is the only one using its own configuration file.

the mbox plugin set the following experiment specific metadata:

* file.size
* mail.subject
* mail.description
* ptr.%d.href
* ptr.%d.role
