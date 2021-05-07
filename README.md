# prospect

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
* **model** (string):
* **source** (string):
* **owner** (string): owner of the data stored in the archive
* **relative-root** (string):
* **acqtime** (date/datetime): a default acquisition time to use for all data files if no acquisition time can be extracted from their content
* **modtime** (date/datetime): a default modification time to use for all data files if no modification time can be extracted from their content
* **metadata**: list of metadata object that will be added to all the data files that are registered in the file section.
  * **name** (string): the name of the metadata
  * **value** (string/bool/date/datetime/float/int): the value associated to the metadata
* **increment**: list of increment during which an experiment take place
  * **increment** (string): label for an increment
  * **starts** (date/datetime): start time of an increment
  * **ends** (date/datetime): end time of an increment
* **file**:
  * **experiment** (string): name of an experiment. if empty, the one of the main section will be used
  * **file** (string): path to a file or directory where data files should be added to the archive
  * **mime** (string):
  * **type** (string):
  * **level** (int): level of processing of the files (default to 0)
  * **acqtime** (date/datetime): default acquisition time to used if no acquisition time can be extracted from their content
  * **modtime** (date/datetime): default modification time to used if no modification time can be extracted from their content
  * **link** (string): kind of link to create between the original data file and the file placed into the archive. Supported values are: *hard*, *sym*, *soft*, *symbolic*.
  * **crews** (list of string): list of crews involved in the experiment.
  * **increments** (list of string): list of increment during which the increment take place.
  * **archive** (string):
  * **extensions** (list of string): list of file extensions that a command will look for in order to accept or reject the file. If a file has an extension that does not appears in the list, a command can discard the file and not process it. If the list is empty, all the files will be accepted.
  * **timefunc** (string):
  * **mimetype**:
    * **extensions** (list of string):
    * **mime** (string):
    * **type** (string):
  * **metadata**: list of metadata object that will be added to all the files found for a specific file section. if metadata are defined in the top level object, they will be merge to this list.
    * **name** (string): name of the metadata
    * **value** (string/bool/date/datetime/float/int): value related to this metadata
  * **links**:
    * **file** (string):
    * **role** (string):

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

the mkfile can be used for any type of files.

a sample configuration for the mkfile command

```toml
file       = "Calibrated/Commands"
mime       = "application/json"
type       = "commands history logs"
level      = 1
timefunc   = "year.doy"
archive    = "{source}/{level}/{type}/{year}"
extensions = [".json", ".gz", ".json.gz"]
```

### mkhdk

the mkhdk command can process all files available in the hadock archive.

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

### mkicn

the mkicn process Inter-Console Note file and their related "data files"

### mkmma

the mkmma command is like the mkcsv command but only for csv files with MMA data inside.

the mkmma command adds the following metadata:

* file.numrec
* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))
* csv.%d.header
* scienceRun.%d
* scienceRun.%d.numrec

### mkmov

the mkmov command can be used to process MOV file (quicktime format)

the mkmov command adds the following metadata:

* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))

### mknef

the mknef command can be used to process images in the NEF format (Nikon Electronic file -
 a kind of tiff file with specific information from Nikon devices).

the mkmma command adds the following metadata:

* file.width
* file.height

### mkpdf

the mkpdf can be used for PDF files.

the mkpdf command adds the following metadata:

* file.author
* file.subject
* file.title
* file.%d.keyword

### mkrt

the mkrt command has been written to process RT files available in the HRDP archive.

the mkrt command adds the following metadata:

* file.numrec
* file.duration ([in ISO format](https://en.wikipedia.org/wiki/ISO_8601#Durations))

### mdexp

the mdexp command, like the mkarc, is not linked to any kind of products. It's main role is to generate the experiment metadata file.

see [configuration for mdexp command](#configuration-for-mdexp-command) for a sample configuration file
