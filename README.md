# prospect

prospect is used to generate the data archive of the different experiments before
transferring these archives to SDC.

prospect delegates most of the work to its [plugins](#Plugins) in order to create
the metadata for each data files to be included in the archive. Then, each data
files are copied by prospect into the archive in their final location.

Each plugin is written either for a specific (eg: rt, hadock) or a generic (eg:
csv, basic, mbox) file format. See the section on plugins to have the list of
of already available [plugins](#Plugins).

Depending on its configuration, propsect will create its archive directly on the
filesystem or in a zip archive. Moreover, it is possible via special [pattern](#Path-pattern)
to decide where each individual data files will be stored in the archive.

To execute prospect, the following command can be used:

```sh
usage:

$ prospect [-s schedule] config.toml
```

## Configuration

the configuration file and its structure of prospect is described in the sections that
follow. More information on the TOML format can be found [here](https://github.com/toml-lang/toml)

### root level

* archive: directory where data files are metadata files (or zip file) will be created
* no-data: tell prospect to only generate the metadata file
* path   : [path pattern](#Path-generation) used to build the final path of data files in the archive

### table [meta]

the meta table (and its sub tables) groups all properties that describes the experiment
itself. The options are used to generate the MD_EXP_<experiment>.xml file.

* acronym     : name of the experiment
* experiment  : full name of the experiment
* id          : erasmus experiment ID - not used
* dtstart     : start date of the experiment
* dtend       : end date of the experiment
* fields      : list of research fields
* coordinators: list of people invovled in the experiments
* increments  : list of increments (start-end)

#### table [[meta.payload]]

* name   : full name of the payload
* acronym: acronym of the payload - not used
* class  : class of the payload (1, 2, 3)

### table [dataset]

* rootdir  : not used
* owner    : dataset owner
* level    : processing level of the dataset (default to 0)
* integrity: [hash algorithm](#Supported-hash-algorithms) to be used to compute the checksum of the data files
* model    : source having generating the dataset

### [[module]]

* module  : path to the plugin/module to be loaded by prospect
* location: [path](#Globbing) to data files
* type    : product type of the data files
* mime    : file format of the data files
* path    : [path pattern](#Path-generation) used to build the final path of the file in the archive
* level   : product level (default: 0)
* config  : plugin specific configuration file
* acqtime : algorithm to be used to compute the acquisition time of data files

notes:

* the type and mime option even if set, can be ignored by the plugin implementation.
* the level option even if set, can be ignored by the plugin implementation.
* the config option even if set, can be ignored by the plugin implementation.
* the acqtime option even if set, can be ignored by the plugin implementation.

#### [[module.mimetype]]

* extension: list of extensions (prefixed with a dot)
* mime     : mime type to be set for the given list of extension
* type     : product type matching the extension and the mime type

## schedule

A optional configuration file can also be given to prospect. This second
configuration file provides to prospect a list of activities that have been
performed in a given time range. This schedule contains the [kind](#Data-source)
of activities that have been performed and which [source](#Model) has generated the data.

This list of activities should be given in a CSV file. this file should have the
following fields (in the given order):

* activity start time (RFC3339 format)
* activity end time (RFC3339 format)
* activity type
* activity comment - should be present even if empty

Note that prospect does not consider the first line as being header.

When a data file can be linked to one activity, prospect will use the values of
a matching activity to add specific experiment metadata to this file.

| metdata | description |
| :---    | :---        |
| activity.dtstart | start time of an activity              |
| activity.dtend   | end time of an activity                |
| activity.desc    | only if the field comment is not empty |

Moreover, files that can not be linked to any will be discarded by prospect and,
as consequence, won't be saved into the final archive.

## Enumerations

Bellow you will find enumerations for some properties. These list are not exhaustives
and prospect does not check that the values set for these properties matches
the values given in the enumerations.

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

## Globbing

pattern can be passed to the location option in the module tables. This pattern
have the following syntax:

* c: match the character literally
* ?: match a single character
* *: match zero or multiple characters
* [a-zA-Z0-9]: match a character in the given set of characters
* **: match any levels of sub directories
* !(foo|bar): negate a matching
* @(foo|bar): match alternative
* ?(ab|cd): match zero or one time the given pattern
* +(ab|cd): match at least one time the given pattern
* *(ab|cd): match zero or multiples time the given pattern

examples:
```sh
var/hadock/main/l0/OPS/images/**/*.*!(.xml)
tmp/mails/dors/**/*.@(txt|pdf|xlsx)
tmp/mails/FSL-@(DORS|ARS)
```

## Supported hash algorithms

prospect can generate the checksum of data files with the following algorithms:

* MD5
* SHA-1
* SHA-256
* SHA-512

## Path generation

In order to control the final location of files in the archive, prospect uses a
parameterizable path pattern via the {} notation. The parameters used in this
pattern will be used to create the final path.

The following properties can be used:

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

Note that any "propreties" not listed above will be injected by prospect as is in
the final path. Moreover, literal string can be used as part of the path and will
be kept as is by prospect to create the final path.

examples:

```
model = Flight Model
date  = 2020-02-18 15:40:05
type  = Uplinks

{model}/DORS/{year}/{doy}/ => FlightModel/DORS/2020/049/
{model}/{type}/ICN/{year}/{month}/{day} => FlightModel/Uplinks/ICN/2020/02/18/
```

## Plugins

### Plugin interfaces

Plugins to be loaded by prospect should be written in go and should have the
following interface:

```go
func New(Config) Module

func New(Config) (Module, Error)

type Module interface {
	Process() (FileInfo, error)
	fmt.Stringer
}
```

### basic plugin

The basic plugin read its data files as is without trying to perform any kind of
logic on the data found in the files (hence its name).

It can be used for any kind of files like:

* images (png/jpg)
* text files (xml, json, csv)
* and many others

The basic plugin is recommended when the data format in a file is unknown or when
we consider the file to be processed as a black box.

This plugin expect having its location option set to a [pattern](#Globbing) to find
the files to be processed

The basic plugin set the following experiment specific metadata to each of its data
files:

| metdata | description |
| :---    | :---        |
| file.size | total size of a file (in bytes) |

If no mime types are set in the module config or none match, the plugin set the mimetype
property to **application/octet-stream**

If no type is set in the module config, the plugin set the type property to **data**

### rt plugin

The rt plugin has been specifically designed to process files found in the HRDP
archive - the so called RT files. These files are used to store:

* Medium rate telemetry (path telemetry)
* Processed parameters
* high rate data

These three kind of files are organized in the same way:

* 4 bytes (little endian encoding) given the length of the packet that follow
* data packet

This plugin expect having its location option set to a [pattern](#Globbing) to find
the files to be processed.

The rt plugin set the following experiment specific metadata:

| metdata | description |
| :---    | :---        |
| file.duration  | 300s |
| file.numrec    | number of raw packets found in a file |
| file.size      | total size of a file (in bytes) |
| file.corrupted | information found in the size header is invalid |

If no mime types are set in the module config or none match, the plugin set the mimetype
property to **application/octet-stream;access=sequential,form=unformatted**

If no type is set in the module config, the plugin set the type property to
**medium rate telemetry**

### hadock plugin

The hadock plugin has been implemented to deal specifically with the data files
avaiable in the archive filled by the Hadock software use in the frame of FSL
activities.

This plugin can be used either for science data or image data or both. However,
there is an important difference in the metadata that will be generated between
science data and image data. Indeed, most of the metadata will be taken from
the XML files that are created by Hadock when saving images in its archive.
Because science data don't have this XML files, they won't have as many experiment
specific metadata than images.

Additional notes:

* this plugin can not be used to process files that have not the file format
  used by Hadock to store its L0 files.
* due to how some sources are used by the VMU (MMA, MVIS), it is not possible for
  the plugin to generate more metadata for these sources. The same limitation
  already exists for Hadock

The hadock plugin set the following experiment specific metadata:

| metdata | description |
| :---    | :---        |
| file.size | total size of a file (in bytes) |
| hpkt.vmu2.hci | channel identifier  |
| hpkt.vmu2.origin | originator identifier |
| hpkt.vmu2.source | originator identifier |
| hpkt.vmu2.upi | user provided information |
| hpkt.vmu2.instance | OPS, SIM1 SIM2, TEST |
| hpkt.vmu2.mode | realtime, playback |
| hpkt.vmu2.fmt | image format information |
| hpkt.vmu2.pixels.x | number of pixels in X axis |
| hpkt.vmu2.pixels.y | number of pixels in Y axis |
| hpkt.vmu2.invalid | computed checksum mismatched checksum of packet |
| hpkt.vmu2.roi.xof | region of interest X offset |
| hpkt.vmu2.roi.xsz | region of interest X size |
| hpkt.vmu2.roi.yof | region of interest Y offset |
| hpkt.vmu2.roi.ysz | region of interest Y size |
| hpkt.vmu2.fdrp | frame dropping |
| hpkt.vmu2.scale.xsz | scaling configuration X size |
| hpkt.vmu2.scale.ysz | scaling configuration Y axis |
| hpkt.vmu2.scale.far | force aspect ratio |


Note that the hpkt.vmu2.* metadata are only given when the data file contains
only data of an image.

If no mime types are set in the module config or none match, the plugin set the mimetype
property to **application/octet-stream**

If no type is set in the module config, the plugin set the type property to
**high rate data**

### icn plugin

The icn plugin is to be used to create the metadata for the inter-console note and
for the files having been uplinked.

The icn plugin set the following experiment specific metadata for inter-console note:

| metdata | description |
| :---    | :---        |
| file.size | total size of a file |
| file.numrec | number of uplinked files found in the ICN |
| ptr.%d.href | path to data file referenced in the ICN |
| ptr.%d.role | uplinked file |

If no mime types are set in the module config or none match, the plugin set the mimetype
property to **text/plain;access=sequential;form=block-formatted;type=icn**

If no type is set in the module config, the plugin set the type property to
**inter-console note**

The icn plugin set the following experiment specific metadata for uplinked files:

| metdata | description |
| :---    | :---        |
| file.size | size of a file as given in the ICN |
| file.md5 | MD5 checksum of a file as given in the ICN |
| uplink.file.local | local filename |
| uplink.target.path | filename used after uplink |
| uplink.time.uplink | schedule time of uplink as given in the ICN |
| uplink.time.transfer | schedule time of transfer as given in the ICN |
| ptr.%d.href | path to ICN file |
| ptr.%d.role | inter-console note |

If no mime types are set in the module config or none match, the plugin set the mimetype
property to **text/plain**

If no type is set in the module config, the plugin set the type property to
**uplink file**

The icn plugin expects having as only input file (specified in the location option)
a path to a CSV file with the following fields (in the given order):

* Source ICN
* Uplinked file
* Original filename
* Command filename
* Filename used for uplink
* Source Id
* Uplink time
* Transfer time
* Warning
* File size
* File MD5

This kind of file can be generated thanks to the script scripts/icn.awk

The icn plugin does not consider the first line as being header.

### csv plugin

The csv plugin, as its name suggests, process only CSV files. However, it consider
the fields and records of its files as blackbox. The plugin has only two expectations
on the format of its files. First, every records in the input files should contain
exactly the same number of fields. And, second, the first row of each files should
contain the headers for each column.

The csv plugin set the following experiment specific metadata:

| metdata | description |
| :---    | :---        |
| file.size | total size of a file (in bytes) |
| file.duration | 300s |
| file.numrec | number of records |
| csv.%d.header | header x of file |

If no mime types are set in the module config or none match, the plugin set the mimetype
property to **text/csv**

If no type is set in the module config, the plugin set the type property to
**data**

### mbox plugin

The mbox plugin extracts its files from e-mails and their attachment found in mbox
files. Each e-mail are parsed individually and their parts are filtered according
to the plugin specific configuration file.

#### plugin configuration

##### table [[mail]]

* type: set the product type property for a matching file
* prefix: add the given prefix to each e-mail when the body should be saved
* maildir: directory where e-mails or their attachments will be saved before being
  added to the archive
* metadata: content-type of a part of an e-mail to be used as specific metadata

###### table [mail.predicate]

This table configures the filters that will be used by the plugin to keep or discard
e-mail.

* from: sender of e-mail
* to: receiver of e-mail
* no-reply: discard all e-mail that have the header In-Reply-To set
* subject: regular expression to match with the subject of an e-mail
* dtstart: e-mail should have been send after the given date
* dtend: e-mail should have been send before the given date
* attachment: e-mail should have at least one attachment. If set to false e-mail
  having or not attachments will be kept

###### table [[mail.file]]

This table configures which parts of e-mail should be include in the archive.

* role: set the value of the ptr.%d.role specific experiment metadata
* pattern: regular expression to match with the filename of an attachment
* content-type: list of content-type to match in order of preference. As soon as
  a match is found, no other part are looking for.

The mbox plugin set the following experiment specific metadata:

| metdata | description |
| :---    | :---        |
| file.size | total size of a file (in bytes) |
| mail.subject | subject of an e-mail |
| mail.description | body of an e-mail (if configured) |
| ptr.%d.href | pointer to related files |
| ptr.%d.role | attachment or e-mail |

Note that the product type and mimetype properties are with the values given in
the configuration file of the plugin.

If the product type is still empty, then it is the value given in the type option
of the module that is used or the value is set to **data**.
