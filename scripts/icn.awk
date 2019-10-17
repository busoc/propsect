#!/usr/bin/awk -f

function trimSpace(value) {
  gsub(/^\s|\s$/, "", value);
  return value
}

function parseFilename(str) {
  match(str, /S_([0-9]+)_/)

  info["sid"] = substr(str, RSTART, RLENGTH)
  gsub(/S_|_/, "", info["sid"])

  str = substr(str, RSTART+RLENGTH)

  match(str, /[A-Z0-9_]+_FILE_[0-9]{3}(_DAT)?_/)
  info["slot"] = substr(str, RSTART, RLENGTH)
  gsub(/^_|_$/, "", info["slot"])

  str = substr(str, RSTART+RLENGTH)

  match(str, /_[0-9]{2}_[0-9]{3}_[0-9]{2}_[0-9]{2}/)
  info["day"] = "20"substr(str, RSTART+1, 6)
  gsub(/_/, "/", info["day"])

  info["origin"] = substr(str, 0, RSTART)
  gsub(/^_|_$/, "", info["origin"])
}

function dump(i) {
  ori = data[i]["origin"]
  gsub(/File_/, "", ori)

  if (ori == "") {
    return
  }

  cmd = data[i]["command"]
  gsub(/File_/, "", cmd)

  slot = data[i]["slot"]
  gsub(/_DAT$/, ".DAT", slot)

  file = data[i]["filename"]
  size = data[i]["size"]
  cksum = data[i]["checksum"]

  day = data[i]["day"]
  uplink = data[i]["uplink"] == "" ? "" : day" "data[i]["uplink"]
  transfer = data[i]["transfer"] == "" ? "" : day" "data[i]["transfer"]

  flag = data[i]["flag"] == "" ? "-" : "*"

  # print FILENAME, file, ori, cmd, slot, uplink, transfer, size, cksum
  printf("%32s | %32s | %36s | %14s | %14s | %s | %8s | %s\n", ori, cmd, slot, uplink, transfer, flag, size, cksum)
}

BEGIN {
  FS=":"
  OFS = ","
}
/Title/ {
  line = $0

  match(line, /[0-9]{2}:[0-9]{2}/)
  uplinkTime = substr(line, RSTART, RLENGTH)

  line = substr(line, RSTART+RLENGTH)

  match(line, /[0-9]{2}:[0-9]{2}/)
  transferTime = substr(line, RSTART, RLENGTH)
}
/File [Ss]ize/ {
  gsub(/\s*(bytes|,)\s*/, "", $2)
  data[file]["size"] = trimSpace($2);
}
/Filename/ {
  file = trimSpace($2)

  data[file]["uplink"] = uplinkTime
  data[file]["transfer"] = transferTime
  data[file]["filename"] = file;

  parseFilename(file)
  data[file]["slot"] = info["slot"]
  data[file]["sid"] = info["sid"]
  data[file]["origin"] = info["origin"]
  data[file]["day"] = info["day"]

  command = info["origin"]
  gsub(/_?[vV][0-9]+.*$/, "", command)
  data[file]["command"] = command

  if (warning != "" ) {
    data[file]["flag"] = "*"
  }
}
/Checksum/ {
  data[file]["checksum"] = trimSpace($2);
}
/\[b\]/  {
  warning = "*"
}
END {
  for (i in data) {
    dump(i)
  }
}
