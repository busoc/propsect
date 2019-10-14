#!/usr/bin/awk -f

function trimBlank(value) {
  v = value
  gsub(/^( |\n)/, "", v);
  return v
}

BEGIN {
  FS=":"
  OFS = ","
  # put the list of slot here (from hourglass database)
  slots[318826020] = "TMLOG4.DAT";
  slots[318826018] = "TMLOG2.DAT";
  slots[318826019] = "TMLOG3.DAT";
  slots[318826006] = "FWPATCH.BIN";
  slots[318826016] = "SUNTIME.IOP";
  slots[318826017] = "TMLOG1.DAT";
  slots[318826005] = "IOP11.IOP";
  slots[318826011] = "TMLOG.DAT";
  slots[318826007] = "SWPATCH.VLF";
  slots[318826012] = "POSTMORT.DAT";
  slots[318826014] = "SAATIME.IOP";
  slots[318826010] = "MMIASPR.DAT";
  slots[318826015] = "SUNSBS.IOP";
  slots[278760447] = "PL_FILE_ISPR_O1_SUB2_FILE_002.DAT";
  slots[278760448] = "PL_FILE_ISPR_O1_SUB2_FILE_003.DAT";
  slots[278760449] = "PL_FILE_ISPR_O1_SUB2_FILE_004.DAT";
  slots[278760450] = "PL_FILE_ISPR_O1_SUB2_FILE_005.DAT";
  slots[278760451] = "PL_FILE_ISPR_O1_SUB2_FILE_006.DAT";
  slots[318826008] = "MMIA.DAT";
  slots[318826009] = "MXGS.DAT";
  slots[318826002] = "SPRSBS.IOP";
  slots[318826013] = "SAASBS.IOP";
  slots[278760438] = "PL_FILE_ISPR_O1_SUB1_FILE_003.DAT";
  slots[278760439] = "PL_FILE_ISPR_O1_SUB1_FILE_004.DAT";
  slots[278760440] = "PL_FILE_ISPR_O1_SUB1_FILE_005.DAT";
  slots[278760441] = "PL_FILE_ISPR_O1_SUB1_FILE_006.DAT";
  slots[278760442] = "PL_FILE_ISPR_O1_SUB1_FILE_007"; #.DAT";
  slots[278760443] = "PL_FILE_ISPR_O1_SUB1_FILE_008.DAT";
  slots[278760444] = "PL_FILE_ISPR_O1_SUB1_FILE_009.DAT";
  slots[278760446] = "PL_FILE_ISPR_O1_SUB2_FILE_001.DAT";
  slots[278760445] = "PL_FILE_ISPR_O1_SUB1_FILE_010.DAT";
  slots[318826003] = "IOPDEF.IOP";
  slots[278760436] = "PL_FILE_ISPR_O1_SUB1_FILE_001.DAT";
  slots[278760437] = "PL_FILE_ISPR_O1_SUB1_FILE_002.DAT";
  slots[318826000] = "MMIASBS.IOP";
  slots[318797801] = "FSL_FILE_002"; #.DAT";
  slots[318797804] = "FSL_FILE_005"; #.DAT";
  slots[318797803] = "FSL_FILE_004"; #.DAT";
  slots[318797802] = "FSL_FILE_003"; #.DAT";
  slots[318797800] = "FSL_FILE_001"; #.DAT";
  slots[318826001] = "MXGSSBS.IOP";
  slots[318826004] = "TIMEDEF.IOP";
  slots[278760460] = "PL_FILE_ISPR_O1_SUB3_FILE_005.DAT";
  slots[278760458] = "PL_FILE_ISPR_O1_SUB3_FILE_003.DAT";
  slots[278760463] = "PL_FILE_ISPR_O1_SUB3_FILE_008.DAT";
  slots[278760457] = "PL_FILE_ISPR_O1_SUB3_FILE_002.DAT";
  slots[278760459] = "PL_FILE_ISPR_O1_SUB3_FILE_004.DAT";
  slots[278760452] = "PL_FILE_ISPR_O1_SUB2_FILE_007.DAT";
  slots[278760456] = "PL_FILE_ISPR_O1_SUB3_FILE_001.DAT";
  slots[278760462] = "PL_FILE_ISPR_O1_SUB3_FILE_007.DAT";
  slots[278760461] = "PL_FILE_ISPR_O1_SUB3_FILE_006.DAT";
  slots[278760455] = "PL_FILE_ISPR_O1_SUB2_FILE_010.DAT";
  slots[278760465] = "PL_FILE_ISPR_O1_SUB3_FILE_010.DAT";
  slots[278760454] = "PL_FILE_ISPR_O1_SUB2_FILE_009.DAT";
  slots[278760453] = "PL_FILE_ISPR_O1_SUB2_FILE_008.DAT";
  slots[278760464] = "PL_FILE_ISPR_O1_SUB3_FILE_009.DAT";
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
/File Size/ {
  gsub(/ *bytes|,/, "", $2)
  data[file]["size"] = trimBlank($2);
}
/Filename/ {
  gsub(/\.dat|\.DAT/, "", $2)
  file = trimBlank($2)

  data[file]["uplink"] = uplinkTime
  data[file]["transfer"] = transferTime
  data[file]["filename"] = file;

  split(file, parts, "_")

  sid = parts[2]
  slot = sid in slots ? slots[sid] : sid
  data[file]["slot"] = slot

  ix = index(file, slot) + length(slot)

  origin = substr(file, ix+1)
  origin = substr(origin, 0, length(origin)-13)
  data[file]["origin"] = origin

  command = origin
  gsub(/_?[vV][0-9]+.*$/, "", command)
  data[file]["command"] = command

  len = length(parts)
}
/Checksum/ {
  data[file]["checksum"] = trimBlank($2);
}
END {
  for (i in data) {
    ori = data[i]["origin"]
    cmd = data[i]["command"]
    slot = data[i]["slot"]
    file = data[i]["filename"]
    size = data[i]["size"]
    cksum = data[i]["checksum"]
    uplink = data[i]["uplink"]
    transfer = data[i]["transfer"]
    print FILENAME, file, ori, cmd, slot, uplink, transfer, size, cksum
    # printf("%32s | %32s | %36s | %12s | %12s | %8s | %s\n", ori, cmd, slot, uplink, transfer, size, cksum)
  }
}
