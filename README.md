# prospect

prospect is a library and a set of tools made to create archive of experiments that will have to be transfered to the Science Data Centre.

prospect, in its library, provides a set of Type and Function to generate the metadata required by SDC. It also provides a set of commands that generates the metadata for specific type of data.

## configuration for mk\*\*\* commands

see below for an example of configuration file

```toml
datadir    = "/archive/demo/SDC/data"
metadir    = "/archive/demo/SDC/metadata"
experiment = "demo"
model      = "Flight Model"
source     = "Science Run"
owner      = "European Space Agency"

[[increment]]
increment = "980-981"
starts    = 2020-07-18T00:00:00Z
ends      = 2021-02-23T23:59:59Z

[[increment]]
increment = "982-983"
starts    = 2021-02-24T00:00:00Z
ends      = 2021-10-16T23:59:59Z

[[file]]
file       = "/demo/PTH"
mime       = "application/octet-stream;access=sequential,form=unformatted,type=pth"
type       = "medium rate telemetry"
level      = 0
archive    = "{source}/{level}/{type}/{year}/{doy}/{hour}"
extensions = [".dat"]

[[file]]
file    = "/demo/LineCamera/Calibrated"
mime    = "image/png"
type    = "line camera image"
level   = 1
archive = "{source}/{level}/{type}/{year}/{doy}/{hour}/{min}"

[[file]]
file    = "/demo/SyncUnit/Raw"
mime    = "application/octet-stream;type=hpkt-vmu2,subtype=science"
type    = "smd sync unit"
level   = 0
archive = "{source}/{level}/{type}/{year}/{doy}/{hour}/{min}"

```

* **datadir** (string): path to the directory where the data files will be stored. See also the link option of the **file** section.
* **metadir** (string): path to the directory where the metadata file will be stored.
* **experiment** (string): name of an experiment
* **model** (string): model that has generated the data that will be stored into the archives (flight model, ground model,...)
* **source** (string): type of activities that has generated the data that will be stored into the archive (science run, EST, commissionning).
* **owner** (string): owner of the data stored in the archive
* **relative-root** (string): a string that will be added to the relativePath element of each product
* **acqtime** (date/datetime): a default acquisition time to use for all data files if no acquisition time can be extracted from their content
* **modtime** (date/datetime): a default modification time to use for all data files if no modification time can be extracted from their content
* **include** (string): path to a file that contains common values for options that can be reused for multiple file section. The included file can only contain options describe just above
* **metadata**: list of metadata object that will be added to all the data files that are registered in the file section. This option allows to specify metadata that are commons to all data files that can be extracted from the content of the files that will be stored into the archive
  * **name** (string): the name of the metadata
  * **value** (string/bool/date/datetime/float/int): the value associated to the metadata
* **increment**: list of increment during which an experiment take place
  * **increment** (string): label for an increment
  * **starts** (date/datetime): start time of an increment
  * **ends** (date/datetime): end time of an increment
* **command**: list of commands that can be executed for certain type of data (only available for nef and mov data products). The main purpose is to generated additional metadata that can be extracted from external tools and their results will be added as specific products
    * **path** (string): path to the command to be executed or only the filename
    * **version** (string): option to give to the command to retrieve version information of the command. It will be added as specific metadata to the output of the command
    * **args** (list of string): arguments to give the command
    * **mime** (string): mime type of the output of the command
    * **type** (string): data type of the output of the command
    * **ext** (string): extension to give to file resulting of the output of the command
    * **extensions** (string):
* **file**: a list of file/directory where data files will be extracted and their metadata generated before being stored into the final archive
  * **experiment** (string): name of an experiment. if empty, the one of the main section will be used
  * **file** (string): path to a file or directory where data files should be added to the archive
  * **mime** (string): mime type describing the file format of the product(s) in a given location. Most of the command will use this string in order to determine if they have to discard the section or if they have to process the files of the current section.
  * **type** (string): type of products found in the given location (productType)
  * **level** (int): level of processing of the files (default to 0)
  * **acqtime** (date/datetime): default acquisition time to used if no acquisition time can be extracted from their content
  * **modtime** (date/datetime): default modification time to used if no modification time can be extracted from their content
  * **link** (string): kind of link to create between the original data file and the file placed into the archive. Supported values are: *hard*, *sym*, *soft*, *symbolic*.
  * **crews** (list of string): list of crew members involved in the experiment.
  * **increments** (list of string): list of increment(s) during which the increment take place.
  * **archive** (string): a pattern that will describe the final location of a data file and its related metadata into the archive. See below for the syntax of the pattern.
  * **extensions** (list of string): list of file extensions that a command will look for in order to accept or reject the file. If a file has an extension that does not appears in the list, a command can discard the file and not process it. If the list is empty, all the files will be accepted.
  * **timefunc** (string): the name of function that will be used by the commands to extract the acqtime/modtime of a data file. See below for a list of supported values. If the timefunc function is not set, it will be the responsability of the commands (when they can) to guess the best acquisition and modification time.
  * **mimetype**: a list of mimetype that are acceptable for a specific kind of file
    * **extensions** (list of string): list of accepted extensions
    * **mime** (string): mime type that describes the file format of the products in a given location
    * **type** (string): type of product (possibly overwrite the one defined in the section above)
  * **metadata**: list of metadata object that will be added to all the files found for a specific file section. if metadata are defined in the top level object, they will be merge to this list.
    * **name** (string): name of the metadata
    * **value** (string/bool/date/datetime/float/int): value related to this metadata
  * **links**: list of links to other files in the archive
    * **file** (string): path to a file to be included in the archive and to be linked to the current file
    * **role** (string): role of the linked file regarding the current file being processed

### Supported timefunc

* year.doy: this function supposes that the filename contains the day of year of the acquisiton and the parent directory contains the year of acquisition of the data.
* year.doy.hour: this function supposes that the filename contains the hour of the acquisiton, its parent directory contains the day of year of the acquisition and its grand parent directory the year of the acquisiton
* rt: this function supposes that the acquisition time of a file can be extracted in the same way that rt files are stored into the hrdp archive: \<year\>/\<doy\>/\<hour\>/rt_\<from\>_\<to\>.dat
* hadock, hdk: this function supposes that the acquisition time of a file can be extracted from a filename that has the same structure of a file found in the hadock archive
* now: this function generates a acquisition time equal to the moment when this function is called

### pattern syntax

path in the archive can be configured in the following way:

* element surrounded by curly braces will be replaced by their value
* element not surrounded by curly braces are written as is in the final path

the following elements will be replaced by their equivalent values in the config file:

* **level**: product level
* **source, run**: type of activities (science ru, est, commissionning,...)
* **model**: model that has generated the data (ground model, flight model,...)
* **mime, format**: only the sub type of the mimetype
* **type**: data type of the product
* **year**: year of the acquisition time (4 digits)
* **doy**: day of year of the acquisition time (3 digits)
* **month**: month of the acquisition time (2 digits)
* **day**: day of the month of the acquisition time (2 digits)
* **hour**: hour of the day of the acquisition time (2 digits)
* **min, minute**: minute of the acquisition time (2 digits)
* **sec, second**: second of the acquisition time (2 digits)
* **timestamp**: unix timestamp of the acquisition time (2 digits)

some examples:

```toml

acqtime = 2021-05-11T11:13:20Z
model   = "flight model"
source  = "science run"
mime    = "application/json"
type    = "data"
level   = 1

pattern1 = "archive/exp/{level}/{year}/{doy}"
pattern2 = "{source}/{mime}/{level}/{year}/{doy}/{hour}"
pattern3 = "archive/{model}/{source}/calibrated/{mime}/{year}/{month}/{day}"
```

will produces:

```bash
pattern1 = archive/exp/1/2021/127
pattern2 = ScienceRun/json/1/2021/127/11
pattern3 = archive/FlightModel/ScienceRun/calibrated/json/2021/05/07
```

## Some Tips/Advices

* extract all common options in the same configuration file and include it via the include option
  * datadir
  * metadir
  * experiment
  * model
  * source
  * owner
  * relative-root
  * acqtime
  * modtime
  * metadata
  * increment
* group set of related products into the same configuration file (set kind of products that will be processed by two differents commands or by the same command). Use the include option to extract common options as described in the bullet above
* be consistant in the name of the data type that you use in the configuration file. It should be the same as the one given in the Blank Book.

## configuration for mdexp command

```toml
acronym      = "exp"
experiment   = "fullname of my experiment"
dtstart      = 2015-09-01
dtend        = 2015-09-15
fields       = [
  "data management",
  "archiving system"
]
coordinators = [] # list of persons involved in the experiment
increments   = [
  "19-20",
]
```

## available commands

propsect comes with a set of commands each one written for a specific kind of products.
The name choosen for the command should be enough to know for which kind of products it
has been written but the following section will describe the relation between a command
and its type of product.

Most of those commands can be invoked in the following way (however some of them
have options - a very limited number when options are not enough general to be
specified into the configuration file):

```bash
$ mkXYZ config.toml
```

Note: using a command with a product for which it has not been written could produce unexpected
result.

only the mkfile command can be used for any type of products because it does not try to go further
in the content of the product.

All commands by default will add the following specific metadata:

* file.size
* file.md5
* file.encoding: set to application/gzip if the file is compressed (extension ends with .gz)

### mkarc

the mkarc command is not linked to any products. It's main goal is to have one command
to centralize the execution of the different mk\*\*\* commands made available by prospect.

its only input is a configuration file (see below for an example) that have the following
options/sections:

* **parallel** (integer): number of commands that mkarc will run in parallel. If not given or equal to 0, mkarc will execute all the commands configured simultaneously.

* **command**:
  * **path** (string): name or path to the command that should be executed
  * **file** (string): path to the configuration file to give to the command
  * **args** (list of string): list of options and their arguments to give to the command
  * silent(boolean): cause mkarc to discard the output to stdout and stderr of the current
    command
  * **env** (list of name:value): list of variables that mkarc should be to the environment of
    the executed command
  * **pre** (list of command): list of commands that should be executed sequentially before the
    execution of the current command
  * post(list of command): list of commands that should be executed sequentially after the
    execution of the current command

  in general, only the options **path** and **file** are needed with, in some circumstances, the option **args**.

a sample configuration file

```toml
parallel = 4

[[command]]
path = "mdexp"
file = "etc/prospect/exp/md.toml"
args = ["-d", "/archive/"]

[[command]]
path = "mkmov"
file = "etc/prospect/exp/quicktime.toml"
env  = [
  {name: "QUALITY", value: "LOW"},
  {name: "LENGTH", value: 900},
]

[[command]]
path   = "mknef"
file   = "etc/prospect/exp/images.toml"
silent = true

  [[command.pre]]
  path = "copy.sh"
  args = ["-s", "/tmp/images", "-d", "/archives/images"]

  [[command.pre]]
  path = "hash.sh"
  file = "/archives/images/"

  [[command.post]]
  path = "rm.sh"
  file = "/tmp/images"

[[command]]
path = "mkcsv"
file = "etc/prospect/exp/dump.toml"
```

### mkcsv

the mkcsv command is specialized in the processing of CSV file only.

the mkcsv command only process files from a file section if the mime option is set
to **text/csv**. Moreover, additional parameters can be added to the mimetype string
to help the command to detect the correct delimiter.

eg: `text/csv;delimiter=tab`

the recognized values for the delimiter option are:

* tab
* comma (default if delimiter option is not specifier)
* space
* pipe
* colon
* semicolon

mkcsv expects the following points for each input files:

* the first line contains the headers
* the first column contains time information in the form of yyyy-mm-ddTHH:MM:SS.xxx

this command use the time value of the first row (after the headers) to set the acquisition time and the time value of the last row to set the modification time.

this command add the following specific metadata:

* file.numrec
* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))
* csv.%d.header

a sample configuration for csv file:

```toml
[[file]]
file       = "Calibrated/Dump"
type       = "parameter dump"
mime       = "text/csv;delimiter=tab"
level      = 1
timefunc   = "year.doy"
archive    = "{source}/{level}/{type}/{year}"
extensions = [".csv", ".gz", ".csv.gz"]
```

### mkfile

the mkfile can be used for any type of files. It use the information given in its "file" section to set all the propreties in the metadata

a sample configuration for the mkfile command

```toml
file       = "Calibrated/Commands"
mime       = "application/json"
type       = "commands history logs"
acqtime    = 2021-05-07T08:26:00
modtime    = 2021-05-07T11:26:00
level      = 1
timefunc   = "year.doy"
archive    = "{source}/{level}/{type}/{year}"
extensions = [".json", ".gz", ".json.gz"]
```

### mkhdk

the mkhdk command can process all files available in the hadock archive.

to know if the command should processed a given file section in the config file, the mkhdk command checks the mime option for the following information:

* level option is set to 1 and the mime type equal to image/png or image/jpeg
* mime type is set to application/octet-stream and type parameter is set to hpkt-vmu2 and the subtype parameter is set to image or science

the acquisition time is taken from the time string available in the filename. The modification is set to the acquisition time.

the mkhdk command adds the following metadata:

* hpkt.vmu2.hci
* hpkt.vmu2.origin
* hpkt.vmu2.source
* hpkt.vmu2.upi
* hpkt.vmu2.instance
* hpkt.vmu2.mode
* hpkt.vmu2.fmt
* hpkt.vmu2.pixels.x
* hpkt.vmu2.pixels.y
* hpkt.vmu2.invalid
* hpkt.vmu2.roi.xof
* hpkt.vmu2.roi.xsz
* hpkt.vmu2.roi.yof
* hpkt.vmu2.roi.ysz
* hpkt.vmu2.fdrp
* hpkt.vmu2.scale.xsz
* hpkt.vmu2.scale.ysz
* hpkt.vmu2.scale.far
* scienceRun

example

```toml
[[file]]
file    = "LineCamera/Calibrated"
mime    = "image/png"
type    = "line camera image"
level   = 1
archive = "{source}/{level}/{type}/{year}/{doy}/{hour}/{min}"

[[file]]
file    = "OverviewCamera/Raw"
mime    = "application/octet-stream;type=hpkt-vmu2,subtype=image"
type    = "overview camera image"
level   = 0
archive = "{source}/{level}/{type}/{year}/{doy}/{hour}/{min}"

[[file]]
file    = "SyncUnit/Raw"
mime    = "application/octet-stream;type=hpkt-vmu2,subtype=science"
type    = "smd sync unit"
level   = 0
archive = "{source}/{level}/{type}/{year}/{doy}/{hour}/{min}"
```

the mkhdk accepts one option: -skip-bad. All files in the hadock archive with .bad extension will be discarded.

### mkicn

the mkicn process Inter-Console Note file and their related "data files". In one run, the mkicn will search for files having the .icn extension and foreach entry found in the ICN will read the file specified and includes it into the archive.

The mkicn will only process file section having the mime option set to "text/plain;type=icn". Additional parameters are allowed.

The acquisition time of the file is taken from the uplink time available in the heading of the inter console note.

The mkicn adds the following metadata:

* ptr.%d.href: path to a related file
* ptr.%d.role: role of a related file (inter console note, parameters table)

example

```toml
[[file]]
file       = "ICN"
mime       = "text/plain;type=icn,access=sequential,form=block-formatted"
type       = "inter console note"
extensions = [".icn", ".ICN"]
level      = 1
archive    = "{source}/{level}/{type}/{year}"
```

### mkmma

the mkmma command is like the mkcsv command but only for csv files with MMA data inside.

the mkmma command adds the following metadata:

* file.numrec
* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))
* csv.%d.header
* scienceRun.%d
* scienceRun.%d.numrec

example

```toml
[[file]]
file       = "MMA/Calibrated"
type       = "MMA"
mime       = "text/csv;delimiter=comma"
level      = 1
timefunc   = "year.doy"
archive    = "{source}/{level}/{type}/{year}"
extensions = [".csv", ".gz", ".csv.gz"]
```

### mkmov

the mkmov command can be used to process MOV file (quicktime format).

The mkmov command will only process files describe in section with the mime option set to video/quicktime.

The acquisition time and modification time are extracted from the metadata available in the video files.

the mkmov command adds the following metadata:

* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))

example

```toml

[[command]]
path       = "./bin/movexplore"
version    = "-v"
type       = "command output"
mime       = "text/plain"
ext        = ".txt"
extensions = [".mov", ".MOV"]

[[file]]
# directory where MOV files are stored
file       = "tmp/thor"
type       = "video"
mime       = "video/quicktime"
level      = 1
crews      = ["Andreas Morgensen"]
increments = ["49-50"]
extensions = [".mov", ".MOV"]
archive    = "{source}/{type}/{level}/{year}/{doy}"
```

### mknef

the mknef command can be used to process images in the NEF format (Nikon Electronic file -
 a kind of tiff file with specific information from Nikon devices).

The mkmov command will only process files describe in section with the mime option set to image/x-nikon-nef

 The acquisition time and modification time are extracted from the metadata available in the image files.

the mkmma command adds the following metadata:

* file.width
* file.height

example

```toml
[[command]]
path       = "./bin/nefex"
type       = "command output"
mime       = "text/plain"
ext        = ".txt"
extensions = [".nef", ".NEF"]

[[file]]
# directory where NEF files are stored
file       = "tmp/thor"
type       = "image"
mime       = "image/x-nikon-nef"
level      = 0
increments = ["49-50"]
crews      = ["Andreas Morgensen"]
extensions = [".nef", ".NEF"]
archive    = "{source}/{type}/{level}/{year}/{doy}"
```

### mkpdf

the mkpdf can be used for PDF files.

The mkmov command will only process files describe in section with the mime option set to application/pdf.

The acquisition and modification times are extracted from the metadata available in the pdf document.

the mkpdf command adds the following metadata:

* file.author
* file.subject
* file.title
* file.%d.keyword

example

```toml
[[file]]
file       = "Anomalies"
type       = "Anomalies Reports"
mime       = "application/pdf"
level      = 1
archive    = "{source}/{level}/{type}/{year}"
extensions = [".pdf"]
```

### mkrt

the mkrt command has been written to process RT files available in the HRDP archive.

the mkrt command only process file section having the type option set to (case insensitive):

* Medium Rate Telemetry
* Processed Data
* High Rate Data

the acqusition and modification times are extracted from the directory where the rt files are located (tree structure similar to the one of the HRDP archive)

the mkrt command adds the following metadata:

* file.numrec
* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))

example

```toml
[[file]]
file       = "PTH"
mime       = "application/octet-stream;access=sequential,form=unformatted,type=pth"
type       = "medium rate telemetry"
level      = 0
archive    = "{source}/{level}/{type}/{year}/{doy}/{hour}"
extensions = [".dat"]

[[file]]
file       = "PDH"
mime       = "application/octet-stream;access=sequential,form=unformatted,type=pdh"
type       = "processed data"
level      = 0
archive    = "{source}/{level}/{type}/{year}/{doy}/{hour}"
extensions = [".dat"]

[[file]]
file       = "HRDL"
mime       = "application/octet-stream;access=sequential,form=unformatted,type=hrd"
type       = "high rate data"
level      = 0
archive    = "{source}/{level}/{type}/{year}/{doy}/{hour}"
extensions = [".dat"]
```

### mdexp

the mdexp command, like the mkarc, is not linked to any kind of products. It's main role is to generate the experiment metadata file.

see [configuration for mdexp command](#configuration-for-mdexp-command) for a sample configuration file
