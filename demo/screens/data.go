package main

import (
	"time"

	"github.com/babarot/gist/v2/internal/fakegist"
)

// login is who the page is logged in as.
const login = "voyager"

const day = 24 * time.Hour

type gist struct {
	age         time.Duration
	description string
	public      bool
	files       map[string]string
	comments    []string
}

// fill gives srv the gists of the page: a probe's notes, all made up, and apart
// from those of the recording, so that a change to one does not move the other.
// Oldest first, as a gist added last is the newest.
func fill(srv *fakegist.Server, now time.Time) {
	srv.Login = login
	for _, g := range mine {
		srv.SetClock(now.Add(-g.age))
		made := srv.Add(g.description, g.public, g.files)
		for _, c := range g.comments {
			srv.Comment(made.ID, "deep-field", c)
		}
	}
	srv.SetClock(now.Add(-12 * day))
	srv.Star("deep-field", "Kepler's equation, solved three ways", map[string]string{"kepler.py": kepler})
	srv.Star("kuiper-belt", "Reading a CCSDS frame by hand", map[string]string{"ccsds.md": ccsds})
	srv.Star("lagrange-point", "Halo orbits, in one page", map[string]string{"halo.md": halo, "halo.py": haloPy})
	srv.Star("oort-cloud", "kubectl for a ground station", map[string]string{"kubectl.sh": kubectl})
	srv.Star("heliopause", "Light time to the outer planets", map[string]string{"light_time.csv": lightTime})
	srv.SetClock(now)
}

var mine = []gist{
	{3 * 365 * day, "Solar wind speed by year", true, map[string]string{"solar_wind.csv": solarWind}, nil},
	{700 * day, "Photometry of a single star", true, map[string]string{"photometry.py": photometry, "requirements.txt": requirements}, nil},
	{420 * day, "Heliopause crossing, what the instruments saw", true, map[string]string{"heliopause.md": heliopause}, []string{"The plasma wave part is the best bit of this."}},
	{300 * day, "Parse two-line elements", true, map[string]string{"tle.go": tle, "tle_test.go": tleTest}, nil},
	{240 * day, "Archive bucket for the raw frames", false, map[string]string{"main.tf": archiveTF, "variables.tf": archiveVars, "outputs.tf": archiveOutputs}, nil},
	{180 * day, "Hohmann transfer between two orbits", true, map[string]string{"hohmann.go": hohmann}, []string{"Worth adding the plane change.", "And the phase angle for the launch window."}},
	{120 * day, "Telescope pipeline image", false, map[string]string{"Dockerfile": dockerfile, "entrypoint.sh": entrypoint}, nil},
	{90 * day, "Crater survey of the near side", true, map[string]string{"craters.csv": craters, "craters.py": cratersPy, "README.md": cratersReadme}, nil},
	{75 * day, "Passes over Goldstone and Canberra", false, map[string]string{"passes.sql": passes}, nil},
	{60 * day, "Ground station on Kubernetes", false, map[string]string{"deployment.yaml": deployment, "service.yaml": service, "kustomization.yaml": kustomization}, nil},
	{45 * day, "Sync the night's frames from the archive", false, map[string]string{"sync_frames.sh": syncFrames}, nil},
	{38 * day, "Alerts for a link going quiet", false, map[string]string{"alerts.yaml": alerts}, nil},
	{30 * day, "Fit a transit to a light curve", true, map[string]string{"transit.py": transit, "transit.md": transitNotes}, nil},
	{24 * day, "Doppler shift of the carrier", true, map[string]string{"doppler.go": doppler}, nil},
	{19 * day, "Decode a telemetry frame", true, map[string]string{"frame.go": frame, "frame_test.go": frameTest, "go.mod": frameMod}, []string{"The checksum is over the header too, isn't it?"}},
	{14 * day, "Jupiter flyby, day by day", true, map[string]string{"jupiter.md": jupiter}, nil},
	{10 * day, "Local stack for the pipeline", false, map[string]string{"compose.yaml": compose, "prometheus.yml": prometheus}, nil},
	{7 * day, "Nightly reduction in Actions", false, map[string]string{"nightly.yaml": nightly}, nil},
	{5 * day, "Star catalog, the brightest within 10 pc", true, map[string]string{"nearby.sql": nearby, "nearby.csv": nearbyCSV}, nil},
	{3 * day, "Ephemeris of the moons of Saturn", true, map[string]string{"saturn.csv": saturn, "fetch.py": horizons}, nil},
	{2 * day, "Link budget at 160 AU", true, map[string]string{"link.go": link}, []string{"3.4 m at X band still closes? Nice."}},
	{19 * time.Hour, "Lunar survey, the south polar transect", true, map[string]string{"transect.md": transect, "transect.csv": transectCSV}, nil},
	{6 * time.Hour, "Things to check before a burn", false, map[string]string{"burn.md": burn}, nil},
	{time.Hour, "Notes from the mission review", false, map[string]string{"review.md": review}, nil},
}

const solarWind = `year,speed_km_s,density_cm3,temp_k
2019,412,5.1,88000
2020,398,5.4,82000
2021,405,5.2,85000
2022,431,4.9,94000
2023,447,4.6,101000
2024,462,4.3,108000
2025,455,4.5,104000
2026,438,4.8,97000
`

const photometry = `"""Aperture photometry of one star across a night of frames."""
from pathlib import Path

import numpy as np
from astropy.io import fits
from photutils.aperture import CircularAperture, aperture_photometry

STAR = (512.4, 498.1)
RADIUS = 8.0


def flux(path: Path) -> float:
    with fits.open(path) as hdul:
        image = hdul[0].data - np.median(hdul[0].data)
    aperture = CircularAperture(STAR, r=RADIUS)
    return float(aperture_photometry(image, aperture)["aperture_sum"][0])


if __name__ == "__main__":
    frames = sorted(Path("frames").glob("*.fits"))
    for frame in frames:
        print(frame.name, f"{flux(frame):.1f}")
`

const requirements = `numpy==2.3.1
astropy==7.1.0
photutils==2.2.0
`

const heliopause = `# Heliopause crossing

What each instrument saw on the days around it.

## Plasma

- Solar wind speed fell to nothing over 3 days
- The sensor stopped seeing the wind at all

## Cosmic rays

- Galactic rays rose by 9% within a week
- Heliospheric particles dropped by a factor of 100

## Magnetic field

The direction did **not** change, which nobody expected.

| Day | Field (nT) | Rays (per s) |
|---|---|---|
| 0 | 0.42 | 2.1 |
| 3 | 0.48 | 2.2 |
| 7 | 0.55 | 2.4 |
`

const tle = `package tle

import (
	"fmt"
	"strconv"
	"strings"
)

// Elements are the orbit a two-line element set gives.
type Elements struct {
	Name        string
	Inclination float64 // degrees
	RAAN        float64 // degrees
	Period      float64 // minutes
}

// Parse reads the three lines of a set, its name first.
func Parse(name, l1, l2 string) (Elements, error) {
	if len(l2) < 63 {
		return Elements{}, fmt.Errorf("line 2 is %d long", len(l2))
	}
	inc, err := strconv.ParseFloat(strings.TrimSpace(l2[8:16]), 64)
	if err != nil {
		return Elements{}, err
	}
	raan, err := strconv.ParseFloat(strings.TrimSpace(l2[17:25]), 64)
	if err != nil {
		return Elements{}, err
	}
	mm, err := strconv.ParseFloat(strings.TrimSpace(l2[52:63]), 64)
	if err != nil {
		return Elements{}, err
	}
	return Elements{name, inc, raan, 1440 / mm}, nil
}
`

const tleTest = `package tle

import "testing"

func TestParse(t *testing.T) {
	e, err := Parse("ISS", "", "2 25544  51.6416 208.9163 0006317  69.9862 320.4562 15.50377579  1234")
	if err != nil {
		t.Fatal(err)
	}
	if e.Inclination != 51.6416 {
		t.Errorf("inclination is %v", e.Inclination)
	}
}
`

const archiveTF = `resource "aws_s3_bucket" "frames" {
  bucket = "${var.mission}-raw-frames"
}

resource "aws_s3_bucket_lifecycle_configuration" "frames" {
  bucket = aws_s3_bucket.frames.id

  rule {
    id     = "cold-after-a-year"
    status = "Enabled"

    transition {
      days          = 365
      storage_class = "DEEP_ARCHIVE"
    }
  }
}

resource "aws_s3_bucket_versioning" "frames" {
  bucket = aws_s3_bucket.frames.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "frames" {
  bucket = aws_s3_bucket.frames.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "frames" {
  bucket = aws_s3_bucket.frames.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

data "aws_iam_policy_document" "frames" {
  statement {
    sid     = "DenyPlainTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.frames.arn,
      "${aws_s3_bucket.frames.arn}/*",
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "frames" {
  bucket = aws_s3_bucket.frames.id
  policy = data.aws_iam_policy_document.frames.json
}
`

const archiveVars = `variable "mission" {
  type        = string
  description = "The name in front of each bucket"
  default     = "voyager"
}
`

const archiveOutputs = `output "bucket" {
  value = aws_s3_bucket.frames.bucket
}
`

const hohmann = `package orbit

import "math"

// Mu is the standard gravitational parameter of the Sun, in km3/s2.
const Mu = 1.32712440018e11

// Hohmann gives the two burns, in km/s, and the days a transfer takes
// between circular orbits of r1 and r2 km.
func Hohmann(r1, r2 float64) (dv1, dv2, days float64) {
	a := (r1 + r2) / 2
	v1 := math.Sqrt(Mu / r1)
	v2 := math.Sqrt(Mu / r2)
	dv1 = v1 * (math.Sqrt(r2/a) - 1)
	dv2 = v2 * (1 - math.Sqrt(r1/a))
	days = math.Pi * math.Sqrt(a*a*a/Mu) / 86400
	return dv1, dv2, days
}
`

const dockerfile = `FROM python:3.13-slim AS build
RUN pip install --no-cache-dir numpy astropy photutils

FROM python:3.13-slim
COPY --from=build /usr/local/lib/python3.13 /usr/local/lib/python3.13
COPY entrypoint.sh reduce.py /app/
WORKDIR /app
USER 65532
ENTRYPOINT ["/app/entrypoint.sh"]
`

const entrypoint = `#!/bin/sh
set -eu

night="${1:-$(date -u +%F)}"
echo "reducing $night"
exec python reduce.py --night "$night" --out "/data/reduced/$night"
`

const craters = `id,lat,lon,diameter_km,depth_km,name
1,-43.3,-11.2,86,4.8,Tycho
2,9.6,-20.1,93,3.8,Copernicus
3,8.1,-38.0,32,2.7,Kepler
4,23.7,-47.4,40,3.2,Aristarchus
5,-8.9,15.5,100,3.0,Theophilus
6,-21.7,16.2,114,4.0,Cyrillus
7,51.6,-9.3,101,2.2,Plato
8,-58.4,-14.4,225,3.7,Clavius
`

const cratersPy = `"""Depth against diameter for the survey."""
import csv

import numpy as np


def load(path):
    with open(path, newline="") as f:
        rows = list(csv.DictReader(f))
    return (
        np.array([float(r["diameter_km"]) for r in rows]),
        np.array([float(r["depth_km"]) for r in rows]),
    )


diameter, depth = load("craters.csv")
slope, intercept = np.polyfit(np.log10(diameter), np.log10(depth), 1)
print(f"depth = {10 ** intercept:.2f} * D^{slope:.2f}")
`

const cratersReadme = `# Crater survey

Eight craters of the near side, measured from the orbital images.

Run craters.py next to craters.csv for the fit.
`

const passes = `-- The passes of the next week, longest first.
SELECT station,
       start_at,
       end_at,
       end_at - start_at AS length,
       max_elevation_deg
FROM passes
WHERE start_at BETWEEN now() AND now() + interval '7 days'
  AND station IN ('goldstone', 'canberra')
  AND max_elevation_deg > 20
ORDER BY length DESC
LIMIT 20;
`

const deployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: frame-receiver
  labels:
    app: frame-receiver
spec:
  replicas: 3
  selector:
    matchLabels:
      app: frame-receiver
  template:
    metadata:
      labels:
        app: frame-receiver
    spec:
      containers:
        - name: receiver
          image: ghcr.io/voyager/frame-receiver:1.8.2
          ports:
            - containerPort: 7788
              protocol: UDP
            - name: health
              containerPort: 9100
          env:
            - name: STATION
              value: south-ridge
            - name: FRAME_DIR
              value: /data/frames
          readinessProbe:
            httpGet:
              path: /ready
              port: health
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /health
              port: health
            initialDelaySeconds: 15
            periodSeconds: 20
          volumeMounts:
            - name: frames
              mountPath: /data/frames
          resources:
            requests:
              cpu: 500m
              memory: 256Mi
            limits:
              cpu: "1"
              memory: 512Mi
        - name: spool
          image: example.com/ground/spool:1.2.0
          args: ["--dir", "/data/frames", "--keep", "6h"]
          env:
            - name: SCAN_INTERVAL
              value: "30s"
          volumeMounts:
            - name: frames
              mountPath: /data/frames
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
      volumes:
        - name: frames
          emptyDir:
            sizeLimit: 4Gi
`

const service = `apiVersion: v1
kind: Service
metadata:
  name: frame-receiver
spec:
  type: LoadBalancer
  selector:
    app: frame-receiver
  ports:
    - port: 7788
      protocol: UDP
`

const kustomization = `resources:
  - deployment.yaml
  - service.yaml
namespace: ground-station
`

const syncFrames = `#!/usr/bin/env bash
# Bring the night's raw frames down from the archive.
set -euo pipefail

night=${1:-$(date -u -d yesterday +%F)}
dest=/data/raw/$night

mkdir -p "$dest"
aws s3 sync "s3://voyager-raw-frames/$night/" "$dest" \
  --exclude '*' --include '*.fits'

echo "$(find "$dest" -name '*.fits' | wc -l) frames in $dest"
`

const alerts = `groups:
  - name: downlink
    rules:
      - alert: LinkQuiet
        expr: rate(frames_received_total[10m]) == 0
        for: 20m
        labels:
          severity: page
        annotations:
          summary: No frames from {{ $labels.station }} for 20 minutes
      - alert: BitErrorRateHigh
        expr: bit_error_rate > 1e-5
        for: 5m
        labels:
          severity: warn
`

const transit = `"""Fit a box to the dip of a transit."""
import numpy as np
from scipy.optimize import curve_fit


def box(t, depth, center, width):
    inside = np.abs(t - center) < width / 2
    return 1 - depth * inside


def load(path):
    t, flux = np.loadtxt(path, unpack=True)
    if t.size < 3 or not np.all(np.isfinite([t, flux])):
        raise ValueError("need at least three finite samples")
    order = np.argsort(t)
    return t[order], flux[order]


def chi_squared(flux, model, sigma):
    if sigma <= 0:
        raise ValueError("noise must be positive")
    return float(np.sum(((flux - model) / sigma) ** 2))


def search(t, flux, periods, depth, width, sigma):
    if np.any(periods <= width):
        raise ValueError("period must exceed the transit width")
    best = (np.inf, None, None)
    for period in periods:
        for epoch in np.linspace(t[0], t[0] + period, 80, endpoint=False):
            phase = (t - epoch + period / 2) % period - period / 2
            model = box(phase, depth, 0, width)
            score = chi_squared(flux, model, sigma)
            if score < best[0]:
                best = (score, period, epoch)
    return best


t, flux = np.loadtxt("lightcurve.txt", unpack=True)
guess = (0.01, t[np.argmin(flux)], 0.1)
(depth, center, width), _ = curve_fit(box, t, flux, p0=guess)

print(f"depth  {depth:.4f}")
print(f"center {center:.4f} d")
print(f"radius {np.sqrt(depth):.3f} of the star's")

times, values = load("lightcurve.txt")
sigma = max(float(np.std(np.diff(values)) / np.sqrt(2)), 1e-6)
score = chi_squared(flux, box(t, depth, center, width), sigma)
print(f"chi2   {score:.2f}")
periods = np.linspace(0.5, 8.0, 400)
score, period, epoch = search(times, values, periods, depth, width, sigma)
print(f"period {period:.4f} d")
print(f"epoch  {epoch:.4f} d, chi2 {score:.2f}")
`

const transitNotes = `# Transit notes

- A box is enough for a first look
- The depth is the square of the ratio of the radii
- Trapezoid next, for the ingress
`

const doppler = `package link

const c = 299792.458 // km/s

// Shift gives the carrier's shift in Hz at a range rate in km/s, which is
// positive going away.
func Shift(carrierHz, rangeRate float64) float64 {
	return -carrierHz * rangeRate / c
}
`

const frame = `package telemetry

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

// Frame is one transfer frame from the probe.
type Frame struct {
	Spacecraft uint16
	VirtualCh  uint8
	Count      uint8
	Data       []byte
}

// ErrShort marks a frame without a complete header.
var ErrShort = errors.New("telemetry: frame too short")

// Decode reads a frame's header. The data is not copied.
func Decode(b []byte) (Frame, error) {
	if len(b) < 6 {
		return Frame{}, ErrShort
	}
	id := binary.BigEndian.Uint16(b)
	return Frame{
		Spacecraft: id >> 4 & 0x3ff,
		VirtualCh:  uint8(id >> 1 & 0x7),
		Count:      b[2],
		Data:       b[6:],
	}, nil
}

// Checksum covers the header and data, without a trailing checksum field.
// The archive keeps this CRC beside the frame, not inside it.
func Checksum(b []byte) uint32 {
	return crc32.ChecksumIEEE(b)
}

// Walk reads fixed-size frames until the stream ends.
// Data is reused after visit returns; copy it to keep a frame.
// A partial final frame returns io.ErrUnexpectedEOF.
func Walk(r io.Reader, size int, visit func(Frame) error) error {
	if size < 6 {
		return ErrShort
	}
	b := make([]byte, size)
	for {
		_, err := io.ReadFull(r, b)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		f, err := Decode(b)
		if err != nil {
			return err
		}
		if err := visit(f); err != nil {
			return err
		}
	}
}
`

const frameTest = `package telemetry

import "testing"

func TestDecodeShort(t *testing.T) {
	if _, err := Decode([]byte{1, 2}); err != ErrShort {
		t.Errorf("err = %v", err)
	}
}
`

const frameMod = `module github.com/voyager/telemetry

go 1.26
`

const jupiter = `# Jupiter flyby

## Day -30

Cameras on. The disk fills 40 pixels.

## Day -7

Bands are clear. **Io** shows a plume at the limb.

## Day 0

Closest approach at 280,000 km.

- 1,200 frames down
- Radiation within the limits
- Trajectory bent toward Saturn as planned

## Day +14

Long look back at the night side.
`

const compose = `services:
  receiver:
    image: ghcr.io/voyager/frame-receiver:1.8.2
    ports:
      - "7788:7788/udp"
  prometheus:
    image: prom/prometheus:v3.5.0
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
`

const prometheus = `global:
  scrape_interval: 15s

scrape_configs:
  - job_name: receiver
    static_configs:
      - targets: ["receiver:9100"]
`

const nightly = `name: nightly reduction

on:
  schedule:
    - cron: "0 6 * * *"
  workflow_dispatch:

jobs:
  reduce:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-python@v6
        with:
          python-version: "3.13"
      - run: pip install -r requirements.txt
      - run: python reduce.py --night "$(date -u -d yesterday +%F)"
`

const nearby = `-- The brightest stars within 10 parsecs.
SELECT name,
       spectral_type,
       parallax_mas,
       1000.0 / parallax_mas AS distance_pc,
       apparent_mag
FROM stars
WHERE parallax_mas > 100
ORDER BY apparent_mag
LIMIT 10;
`

const nearbyCSV = `name,type,distance_pc,mag
Sirius,A1V,2.64,-1.46
Alpha Centauri,G2V,1.34,-0.27
Vega,A0V,7.68,0.03
Procyon,F5IV,3.51,0.34
Altair,A7V,5.13,0.76
Tau Ceti,G8V,3.65,3.50
`

const saturn = `moon,period_d,semi_major_km,inclination_deg
Mimas,0.94,185539,1.57
Enceladus,1.37,237948,0.01
Tethys,1.89,294619,1.09
Dione,2.74,377396,0.02
Rhea,4.52,527108,0.35
Titan,15.95,1221870,0.33
Iapetus,79.32,3560820,15.47
`

const horizons = `"""Ask an ephemeris service for the moons of Saturn."""
import csv
import sys

import requests

API = "https://ephemeris.example/api"


def fetch(body: str) -> dict:
    res = requests.get(API, params={"body": body}, timeout=30)
    res.raise_for_status()
    return res.json()


writer = csv.writer(sys.stdout)
writer.writerow(["moon", "period_d", "semi_major_km"])
for moon in ("Mimas", "Enceladus", "Titan"):
    e = fetch(moon)
    writer.writerow([moon, e["period"], e["a"]])
`

const link = `package link

import (
	"fmt"
	"io"
	"math"
)

const au = 1.495978707e11 // m

// Band holds the carrier and antenna gains for a trial downlink.
type Band struct {
	Name   string
	GHz    float64
	TxGain float64
	RxGain float64
}

// Bands are sample settings for the probe and a small ground dish.
var Bands = []Band{
	{Name: "S", GHz: 2.3, TxGain: 28, RxGain: 36},
	{Name: "X", GHz: 8.4, TxGain: 39, RxGain: 47},
	{Name: "Ka", GHz: 32.0, TxGain: 50, RxGain: 58},
}

// Budget gives the margin of a downlink in dB.
func Budget(rangeAU, ghz, txDBW, gainDB, rxDB, needDB float64) float64 {
	d := rangeAU * au
	lambda := 3e8 / (ghz * 1e9)
	path := 20 * math.Log10(4*math.Pi*d/lambda)
	return txDBW + gainDB + rxDB - path - needDB
}

// PrintBudget writes one margin per band at the same range and power.
// needDB is the required received power in dBW, including receiver noise.
func PrintBudget(w io.Writer, rangeAU, txDBW, needDB float64) error {
	if rangeAU <= 0 || math.IsNaN(rangeAU) || math.IsInf(rangeAU, 0) {
		return fmt.Errorf("link: invalid range %g AU", rangeAU)
	}
	if _, err := fmt.Fprintf(w, "range %.1f AU, power %.1f dBW\n", rangeAU, txDBW); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "band  GHz   margin dB"); err != nil {
		return err
	}
	for _, band := range Bands {
		margin := Budget(rangeAU, band.GHz, txDBW, band.TxGain, band.RxGain, needDB)
		if _, err := fmt.Fprintf(w, "%-4s %5.1f %9.2f\n", band.Name, band.GHz, margin); err != nil {
			return err
		}
	}
	return nil
}
`

const transect = `# South polar transect

Measured along 0 degrees of longitude, from 80 S to the pole.

## Results

- Elevation falls 4.2 km into the basin
- Permanent shadow from 88.5 S
- Hydrogen rises with the shadow

| Lat | Elev (km) | Sun (%) |
|---|---|---|
| -80 | 1.2 | 42 |
| -81.1 | 0.95 | 38 |
| -82.5 | 0.61 | 33 |
| -83.6 | 0.17 | 28 |
| -85 | -0.4 | 21 |
| -85.9 | -0.81 | 17 |
| -87 | -1.34 | 11 |
| -87.4 | -1.66 | 9 |
| -88 | -2.1 | 6 |
| -88.4 | -2.35 | 4 |
| -89 | -2.71 | 1 |
| -89.5 | -3.0 | 0 |

See transect.csv for every station.

## Sampling

The first seven stations set the route.
Extra samples fill the gaps between them.
Each elevation is relative to the same mean surface.
Sun is the fraction of the observation window with direct light.

## Shadow

The last slope faces away from the Sun.
Light reaches the upper rim for a short part of each orbit.
Below the rim, the floor stays dark through the full window.
Repeat the pass at a different season before marking the boundary.

## Hydrogen

The count rises toward the floor.
The values are estimates from the neutron channel, not core samples.
Use the same background correction for the whole route.
The shaded stations need a second pass with a longer dwell.

## Next pass

- Keep the track on the same longitude
- Take another dark reading before the first station
- Hold the last four stations for twice as long
- Compare the rim height with the east-facing track
`

const transectCSV = `station,lat,elevation_km,illumination,hydrogen_ppm
S01,-80.0,1.20,0.42,48
S01a,-80.4,1.12,0.41,50
S01b,-80.7,1.03,0.39,52
S01c,-81.1,0.95,0.38,54
S01d,-81.4,0.86,0.37,55
S01e,-81.8,0.78,0.36,57
S01f,-82.1,0.69,0.34,59
S02,-82.5,0.61,0.33,61
S02a,-82.9,0.46,0.31,65
S02b,-83.2,0.32,0.30,68
S02c,-83.6,0.17,0.28,72
S02d,-83.9,0.03,0.26,76
S02e,-84.3,-0.12,0.24,80
S02f,-84.6,-0.26,0.23,83
S03,-85.0,-0.41,0.21,87
S03a,-85.3,-0.54,0.20,92
S03b,-85.6,-0.68,0.18,96
S03c,-85.9,-0.81,0.17,101
S03d,-86.1,-0.94,0.15,106
S03e,-86.4,-1.07,0.14,111
S03f,-86.7,-1.21,0.12,115
S04,-87.0,-1.34,0.11,120
S04a,-87.1,-1.45,0.10,126
S04b,-87.3,-1.55,0.10,133
S04c,-87.4,-1.66,0.09,139
S04d,-87.6,-1.76,0.08,145
S04e,-87.7,-1.87,0.07,151
S04f,-87.9,-1.97,0.07,158
S05,-88.0,-2.08,0.06,164
S05a,-88.1,-2.17,0.05,174
S05b,-88.3,-2.26,0.05,184
S05c,-88.4,-2.35,0.04,194
S05d,-88.6,-2.44,0.03,203
S05e,-88.7,-2.53,0.02,213
S05f,-88.9,-2.62,0.02,223
S06,-89.0,-2.71,0.01,233
S06a,-89.1,-2.79,0.01,249
S06b,-89.2,-2.87,0.01,266
S06c,-89.4,-2.94,0.00,282
S07,-89.5,-3.02,0.00,298
`

const burn = `# Before a burn

- [x] Attitude within 0.5 degrees
- [x] Tanks at temperature
- [ ] Ground stations in view
- [ ] Sequence uplinked and read back
- [ ] Abort limits agreed
`

const review = `# Mission review

## Decided

- Turn off the cosmic ray heater, for the power
- Keep the plasma wave instrument on

## Before the next one

- [ ] Work out how long the power lasts
- [x] Move the data rate down to 160 bps
`

const kepler = `"""Solve M = E - e sin E for the eccentric anomaly."""
from math import sin, cos, pi


def newton(M, e, tol=1e-10):
    E = M if e < 0.8 else pi
    while True:
        step = (E - e * sin(E) - M) / (1 - e * cos(E))
        E -= step
        if abs(step) < tol:
            return E


print(newton(1.0, 0.3))
`

const ccsds = `# Reading a CCSDS frame by hand

A transfer frame starts with six bytes.

- Version, 2 bits
- Spacecraft ID, 10 bits
- Virtual channel, 3 bits
- Then the counters

Everything after is the data field.
`

const halo = `# Halo orbits

A halo orbit circles a Lagrange point out of the plane.

- Stays in sight of the Earth
- Needs small burns to stay there
- See halo.py for the first guess
`

const haloPy = `"""The first guess of a halo orbit, in the rotating frame."""
MU = 3.003e-6


def l1(mu=MU):
    return 1 - (mu / 3) ** (1 / 3)


print(f"L1 at {l1():.6f} of the distance")
`

const kubectl = `#!/bin/sh
# What I type most on a pass.
kubectl -n ground-station get pods -o wide
kubectl -n ground-station logs -f deploy/frame-receiver
kubectl -n ground-station rollout restart deploy/frame-receiver
kubectl -n ground-station top pods
`

const lightTime = `planet,au,light_minutes
Jupiter,5.2,43
Saturn,9.5,79
Uranus,19.2,160
Neptune,30.1,250
Pluto,39.5,328
`
