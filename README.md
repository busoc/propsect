# prospect

## configuration for mk\*\*\* commands

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

### mkarc

the mkarc command is not linked to any products. It's main goal is to have one command
to centralize the execution of the different mk\*\*\* commands made available by prospect.

its only input is a configuration file (see below for an example) that have the following
options/sections:

* parallel (integer): number of commands that mkarc will run in parallel. If not given or equal to 0, mkarc will execute all the commands configured simultaneously.

* file:
  * path (string): name or path to the command that should be executed
  * file (string): path to the configuration file to give to the command
  * args (list of string): list of options and their arguments to give to the command
  * silent(boolean): cause mkarc to discard the output to stdout and stderr of the current
    command
  * env (list of name:value): list of variables that mkarc should be to the environment of
    the executed command
  * pre (list of command): list of commands that should be executed sequentially before the
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
  args = ["-s" "/tmp/images", "-d", "/archives/images"]

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

### mkfile

### mkhdk

### mkicn

### mkmma

### mkmov

### mknef

### mkpdf

### mkrt

### mdexp

the mdexp command, like the mkarc, is not linked to any kind of products. It's main role is to generate the experiment metadata file.

see [configuration for mdexp command](#configuration-for-mdexp-command) for a sample configuration file
