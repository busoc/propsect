#!/usr/bin/awk -f

BEGIN {
  origins["0a"] = "ATC"
	origins["0b"] = "REDU"
	origins["0c"] = "MCS"
	origins["e1"] = "MCC-H"
	origins["e2"] = "MCC-M"
	origins["e3"] = "HOSC"

  if (datadir == "") {
    datadir = "/tmp"
  }
  print(datadir)
}
{
  gsub(/"/, "", $3)
  if ( !($3 in codes) ) {
    codes[$3] = source

    ori = substr($3, 0, 2)
    file = origins[ori]
    if (file == "") {
      file = sprintf("umi_%s", ori)
    }
    file = sprintf("%s/%s.csv", datadir, file)
    print $3 >> file
  }
}
