#!/usr/bin/awk -f

function trimBlank(value) {
  v = value
  gsub(/^( |\n)/, "", v);
  return v
}

function parseFilename(str) {
  input = str
  ix = match(str, /S_([0-9]+)_/)

  info["sid"] = substr(str, RSTART, RLENGTH)
  gsub(/S_|_/, "", info["sid"])

  str = substr(str, RSTART+RLENGTH)

  ix = match(str, /[A-Z0-9_]+_FILE_[0-9]{3}(_DAT)?_/)
  info["slot"] = substr(str, RSTART, RLENGTH)
  gsub(/^_|_$/, "", info["slot"])

  str = substr(str, RSTART+RLENGTH)

  ix = match(str, /_[0-9]{2}_[0-9]{3}_[0-9]{2}_[0-9]{2}/)
  info["origin"] = substr(str, 0, RSTART)
  gsub(/^_|_$/, "", info["origin"])
}

BEGIN {
  FS=":"
  OFS = ","
}
/Title/ {
  line = $0

  match(line, /(GMT(0| )|GMT)[0-9]{3}\/[0-9]{2}:[0-9]{2}/)
  uplinkTime = substr(line, RSTART, RLENGTH)
  gsub(/ /, "", uplinkTime)
  gsub(/T0{2,}/, "T0", uplinkTime)

  line = substr(line, RSTART+RLENGTH)
  ix = match(line, /(GMT(0| )|GMT)[0-9]{3}\/[0-9]{2}:[0-9]{2}/)
  if (ix > 0) {
    transferTime = substr(line, RSTART, RLENGTH)
    gsub(/ /, "", transferTime)
    gsub(/T0{2,}/, "T0", transferTime)
  } else {
    transferTime = "-"
  }
}
/File [Ss]ize/ {
  gsub(/ *bytes|,/, "", $2)
  data[file]["size"] = trimBlank($2);
}
/Filename/ {
  gsub(/\.dat|\.DAT/, "", $2)
  file = trimBlank($2)

  data[file]["uplink"] = uplinkTime
  data[file]["transfer"] = transferTime
  data[file]["filename"] = file;

  parseFilename(file)
  data[file]["slot"] = info["slot"]
  data[file]["sid"] = info["sid"]
  data[file]["origin"] = info["origin"]

  command = info["origin"]
  gsub(/_?[vV][0-9]+.*$/, "", command)
  data[file]["command"] = command

  if (warning != "" ) {
    data[file]["flag"] = "*"
  }
}
/Checksum/ {
  data[file]["checksum"] = trimBlank($2);
}
/\[b\]/  {
  warning = "*"
}
END {
  for (i in data) {
    ori = data[i]["origin"]
    gsub(/File_/, "", ori)
    if (ori == "") {
      continue
    }

    cmd = data[i]["command"]
    gsub(/File_/, "", cmd)

    slot = data[i]["slot"]
    gsub(/_DAT$/, ".DAT", slot)

    file = data[i]["filename"]
    size = data[i]["size"]
    cksum = data[i]["checksum"]
    uplink = data[i]["uplink"]
    transfer = data[i]["transfer"]
    flag     = data[i]["flag"] == "" ? "-" : "*"
    # print FILENAME, file, ori, cmd, slot, uplink, transfer, size, cksum
    printf("%32s | %32s | %36s | %12s | %12s | %s | %8s | %s\n", ori, cmd, slot, uplink, transfer, flag, size, cksum)
  }
}
