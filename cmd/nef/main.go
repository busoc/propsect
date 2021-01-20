package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/busoc/prospect"
	"github.com/midbel/exif/mov"
	"github.com/midbel/exif/nef"
	"github.com/midbel/toml"
)

const (
	RoleMeta = "exif"
	RoleNef  = "nef"
	RoleImg  = "image"
	RoleMov  = "video"
	RoleData = "data"
	Ptr      = "ptr.%d.href"
	Role     = "ptr.%d.role"
	SizeX    = "image.x"
	SizeY    = "image.y"
	Duration = "file.duration"
)

const (
	ExtDAT  = ".dat"
	ExtJPG  = ".jpg"
	ExtTXT  = ".txt"
	ExtNEF  = ".NEF"
	ExtMOV  = ".MOV"
	ExtDUMP = ".dump"

	SHA = "SHA256"

	MimeNEF  = "image/x-nikon-nef"
	MimeMOV  = "video/quicktime"
	MimeDUMP = "text/csv;comma=tab"
	TypeNEF  = "raw image"
	TypeExif = "exif tags listing"
	TypeMOV  = "video"
	TypeDUMP = "parameters dump"
)

type FileInfo struct {
	Ext string
	Mime string
	Type string
}

type Settings struct {
	Datadir string `toml:"data"`
	Archive string
	Exif    []string
	Infos []FileInfo `toml:"types"`

	prospect.Meta `toml:"meta"`
	prospect.Data `toml:"dataset"`
}

func Load(file string) (Settings, error) {
	var (
		set Settings
		err error
	)
	if err := toml.DecodeFile(file, &set); err != nil {
		return set, err
	}
	set.Datadir, err = filepath.Abs(set.Datadir)
	if err != nil {
		return set, err
	}
	if err = os.MkdirAll(set.Archive, 0755); err != nil {
		return set, err
	}
	if err = os.Chdir(set.Archive); err != nil {
		return set, err
	}
	set.Archive = ""
	set.Data.Experiment = set.Meta.Name
	return set, nil
}

func main() {
	flag.Parse()

	set, err := Load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := writeMeta(set.Archive, set.Meta); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	err = filepath.Walk(set.Datadir, func(file string, i os.FileInfo, err error) error {
		if err != nil || i.IsDir() {
			return err
		}
		var (
			now     = time.Now()
			ext     = filepath.Ext(file)
		)
		switch ext {
		case ExtMOV:
			err = processMov(file, set.Exif, set.Data)
		case ExtDUMP:
			err = processDump(file, set.Data)
		case ExtNEF:
			err = processFile(file, set.Exif, set.Data)
		default:
			// err = processOther(file, set.Infos, set.Data)
		}
		fmt.Printf("done processing %s (%s)", file, time.Since(now))
		fmt.Println()
		return err
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func processOther(file string, types []FileInfo, data prospect.Data) error {
	ext := filepath.Ext(file)
	x := sort.Search(len(types), func(i int) bool {
		return types[i].Ext >= ext
	})
	if x >= len(types) || types[x].Ext != ext {
		return nil
	}
	data.Info.Mime = types[x].Mime
	data.Info.Type = types[x].Type
	data.Info.Level = 1

	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, r); err != nil {
		return err
	}
	data.Info.File = file
	data.Info.Integrity = SHA
	data.Info.Sum = fmt.Sprintf("%x", digest.Sum(nil))
	return nil
}

func processMov(file string, exif []string, data prospect.Data) error {
	data.Info.Mime = MimeMOV
	data.Info.Type = TypeMOV
	acq, mod, length, err := timesFromMov(file)
	if err != nil {
		return err
	}
	data.Info.AcqTime = acq
	data.Info.ModTime = mod
	data.Info.Level = 1

	var meta []byte
	if len(exif) > 0 {
		args := append(exif, file)
		cmd := exec.Command(args[0], args[1:]...)

		buf, err := cmd.Output()
		if err != nil {
			return err
		}
		meta = buf
	}

	datadir, metadir, err := mkdirAll("", acq)
	if err != nil {
		return err
	}

	if len(meta) > 0 {
		data.Info.Parameters = createParamPtr(1, filepath.Join(datadir, file), RoleMov)
		basename := trimExt(file) + ".exif" + ExtTXT
		d, err := processMeta(filepath.Join(datadir, basename), meta, data)
		if err != nil {
			return err
		}
		if err := writeData(filepath.Join(metadir, basename), d); err != nil {
			return err
		}
		data.Info.Parameters = createParamPtr(1, filepath.Join(datadir, basename), RoleMeta)
	}

	if length > 0 {
		p := prospect.MakeParameter(Duration, length.String())
		data.Info.Parameters = append(data.Info.Parameters, p)
	}

	filename, sum, err := copyFile(file, datadir)
	if err != nil {
		return err
	}
	data.Info.File = filename
	data.Info.Integrity = SHA
	data.Info.Sum = fmt.Sprintf("%x", sum)
	return writeData(filepath.Join(metadir, filepath.Base(file)), data)
}

func processDump(file string, data prospect.Data) error {
	data.Info.Mime = MimeDUMP
	data.Info.Type = TypeDUMP
	acq, mod, length, err := timesFromDump(file)
	if err != nil {
		return err
	}
	data.Info.AcqTime = acq
	data.Info.ModTime = mod
	data.Info.Level = 1

	datadir, metadir, err := mkdirAll("", acq)
	if err != nil {
		return err
	}

	if length > 0 {
		p := prospect.MakeParameter(Duration, length.String())
		data.Info.Parameters = append(data.Info.Parameters, p)
	}

	filename, sum, err := copyFile(file, datadir)
	if err != nil {
		return err
	}
	data.Info.File = filename
	data.Info.Integrity = SHA
	data.Info.Sum = fmt.Sprintf("%x", sum)
	return writeData(filepath.Join(metadir, filepath.Base(file)), data)
}

func timesFromMov(file string) (time.Time, time.Time, time.Duration, error) {
	qt, err := mov.Decode(file)
	if err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	defer qt.Close()

	p, err := qt.DecodeProfile()
	if err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	return p.AcqTime(), p.ModTime(), p.Length(), nil
}

func timesFromDump(file string) (time.Time, time.Time, time.Duration, error) {
	r, err := os.Open(file)
	if err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	defer r.Close()

	rs := csv.NewReader(r)
	rs.Comma = '\t'

	rs.Read()

	var acq, mod time.Time
	for i := 0; ; i++ {
		row, err := rs.Read()
		if i == 0 {
			acq, err = time.Parse("2006-01-02T15:04:05.000", row[0])
			if err != nil {
				return acq, mod, 0, err
			}
		}
		if len(row) == 0 || err != nil {
			break
		}
		mod, err = time.Parse("2006-01-02T15:04:05.000", row[0])
	}
	return acq, mod, mod.Sub(acq), nil
}

func copyFile(file, dir string) (string, []byte, error) {
	r, err := os.Open(file)
	if err != nil {
		return "", nil, err
	}
	defer r.Close()

	w, err := os.Create(filepath.Join(dir, filepath.Base(file)))
	if err != nil {
		return "", nil, err
	}
	defer w.Close()

	var (
		sum = sha256.New()
		rs  = io.TeeReader(r, sum)
	)
	_, err = io.Copy(w, rs)
	return w.Name(), sum.Sum(nil)[:], err
}

func processFile(file string, exif []string, data prospect.Data) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	var (
		origin   time.Time
		basename = trimExt(file)
		metafile = basename + ".exif" + ExtTXT
		params   []prospect.Parameter
		meta     []byte
	)

	if len(exif) > 0 {
		var (
			args = append(exif, file)
			cmd  = exec.Command(args[0], args[1:]...)
		)
		buf, err := cmd.Output()
		if err != nil {
			return err
		}
		meta = buf
	}

	files, err := nef.Decode(r)
	if err != nil {
		return err
	}
	for i := range files {
		when, _ := files[i].GetTag(0x0132, nef.Tiff)
		datadir, metadir, err := mkdirAll("", when.Time())
		if err != nil {
			return err
		}
		ps := createParamPtr(1, filepath.Join(datadir, filepath.Base(file)), RoleNef)
		if len(meta) > 0 {
			ps = append(ps, createParamPtr(2, filepath.Join(datadir, metafile), RoleMeta)...)
		}
		data.Info.ModTime = when.Time()
		data.Info.AcqTime = when.Time()
		data.Info.Parameters = ps
		ds, err := processImage(datadir, basename, files[i], data)
		if err != nil {
			return err
		}
		if err := writeMultiData(metadir, ds); err != nil {
			return err
		}
		params, origin = append(params, appendParams(ds)...), when.Time()
	}

	datadir, metadir, err := mkdirAll("", origin)
	if err != nil {
		return err
	}
	data.Info.ModTime = origin
	data.Info.AcqTime = origin
	data.Info.Parameters = createParamPtr(1, filepath.Join(datadir, filepath.Base(file)), RoleNef)
	d, err := processMeta(filepath.Join(datadir, metafile), meta, data)
	if err != nil {
		return err
	}
	if err := writeData(filepath.Join(metadir, metafile), d); err != nil {
		return err
	}
	if i := len(params); len(meta) > 0 {
		file := filepath.Join(datadir, metafile)
		params = append(params, createParamPtr(i, file, RoleMeta)...)
	}
	data.Info.Parameters = params
	d, err = processNEF(datadir, file, data)
	if err != nil {
		return err
	}
	return writeData(filepath.Join(metadir, filepath.Base(file)), d)
}

func processMeta(file string, meta []byte, data prospect.Data) (prospect.Data, error) {
	if err := ioutil.WriteFile(file, meta, 0644); err != nil {
		return data, err
	}
	data.Info.File = file
	data.Info.Integrity = SHA
	data.Info.Sum = fmt.Sprintf("%x", sha256.Sum256(meta))
	data.Info.Mime = prospect.MimePlainDefault
	data.Info.Type = TypeExif

	return data, nil
}

func processNEF(dir, file string, data prospect.Data) (prospect.Data, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return data, err
	}
	r, err := os.Open(file)
	if err != nil {
		return data, err
	}
	defer r.Close()

	w, err := os.Create(filepath.Join(dir, filepath.Base(file)))
	if err != nil {
		return data, err
	}
	defer w.Close()

	var (
		sum = sha256.New()
		ws  = io.MultiWriter(sum, w)
	)
	size, err := io.Copy(ws, r)
	if err != nil {
		return data, err
	}
	data.Info.Size = int(size)
	data.Info.File = w.Name()
	data.Info.Integrity = SHA
	data.Info.Sum = fmt.Sprintf("%x", sum.Sum(nil))
	data.Info.Mime = MimeNEF
	data.Info.Type = TypeNEF

	return data, nil
}

func processImage(dir, base string, f *nef.File, data prospect.Data) ([]prospect.Data, error) {
	var ds []prospect.Data

	d, err := extractImage(dir, base, f, data)
	if err != nil {
		return nil, err
	}
	ds = append(ds, d)
	for i := range f.Files {
		xs, err := processImage(dir, base, f.Files[i], data)
		if err != nil {
			return nil, err
		}
		ds = append(ds, xs...)
	}
	return ds, nil
}

func extractImage(dir, base string, f *nef.File, data prospect.Data) (prospect.Data, error) {
	data.Info.Type = prospect.TypeImage
	data.Info.Mime = prospect.MimeJPG
	data.Info.Level = 1
	data.Info.Integrity = SHA

	var (
		buf []byte
		err error
		ext string
	)
	if !f.IsSupported() {
		data.Info.Mime = prospect.MimeJPG
		data.Info.Type = prospect.TypeData
		data.Info.Level = 0

		buf, err = writeBytes(f)
		ext = ExtDAT
	} else {
		buf, err = writeImage(f)
		ext = ExtJPG

		cfg, _ := jpeg.DecodeConfig(bytes.NewReader(buf))
		ps := []prospect.Parameter{
			prospect.MakeParameter(SizeX, fmt.Sprintf("%d", cfg.Width)),
			prospect.MakeParameter(SizeY, fmt.Sprintf("%d", cfg.Height)),
		}
		data.Info.Parameters = append(data.Info.Parameters, ps...)
	}
	if err != nil {
		return data, err
	}
	data.Info.Size = len(buf)
	data.Info.Sum = fmt.Sprintf("%x", sha256.Sum256(buf))
	data.Info.File = filepath.Join(dir, base+"_"+f.Filename()) + ext

	if err := ioutil.WriteFile(data.Info.File, buf, 0644); err != nil {
		return data, err
	}
	return data, nil
}

func writeImage(f *nef.File) ([]byte, error) {
	img, err := f.Image()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, nil)
	return buf.Bytes(), err
}

func writeBytes(f *nef.File) ([]byte, error) {
	return f.Bytes()
}

func createParamPtr(i int, value, role string) []prospect.Parameter {
	if i <= 0 {
		i = 1
	}
	var (
		pref = prospect.MakeParameter(fmt.Sprintf(Ptr, i), value)
		prol = prospect.MakeParameter(fmt.Sprintf(Role, i), role)
	)
	return []prospect.Parameter{pref, prol}
}

func appendParams(data []prospect.Data) []prospect.Parameter {
	params := make([]prospect.Parameter, 0, len(data))
	for _, d := range data {
		var (
			role = RoleImg
			i    = len(params)
		)
		if filepath.Ext(d.Info.File) == ExtDAT {
			role = RoleData
		}
		params = append(params, createParamPtr(i+1, d.Info.File, role)...)
	}
	return params
}

func writeMultiData(dir string, data []prospect.Data) error {
	for _, d := range data {
		if err := writeData(filepath.Join(dir, filepath.Base(d.Info.File)), d); err != nil {
			return err
		}
	}
	return nil
}

func writeData(file string, data prospect.Data) error {
	w, err := os.Create(file + ".xml")
	if err != nil {
		return err
	}
	defer w.Close()
	return prospect.EncodeData(w, data)
}

func writeMeta(dir string, meta prospect.Meta) error {
	if err := os.MkdirAll(dir, 0755); dir != "" && err != nil {
		return err
	}
	w, err := os.Create(filepath.Join(dir, fmt.Sprintf("MD_EXP_%s.xml", meta.Accr)))
	if err != nil {
		return err
	}
	defer w.Close()
	return prospect.EncodeMeta(w, meta)
}

func mkdirAll(dir string, when time.Time) (string, string, error) {
	datadir, err := mkdirData(dir, when)
	if err != nil {
		return "", "", err
	}
	metadir, err := mkdirMeta(dir, when)
	if err != nil {
		return "", "", err
	}
	return datadir, metadir, nil
}

func mkdirData(dir string, when time.Time) (string, error) {
	var (
		year = when.Format("2006")
		doy  = when.Format("002")
	)
	dir = filepath.Join(dir, year, doy)
	return dir, os.MkdirAll(dir, 0755)
}

func mkdirMeta(dir string, when time.Time) (string, error) {
	return mkdirData(filepath.Join(dir, "metadata"), when)
}

func trimExt(file string) string {
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}

func trimPath(file, archive string) string {
	file = strings.TrimPrefix(file, filepath.Clean(archive))
	return filepath.Clean(file)
}
