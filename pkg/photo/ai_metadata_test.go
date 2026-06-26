package photo

import (
	"strings"
	"testing"
)

func TestContainsAIMeta(t *testing.T) {
	var cases = []struct {
		Str     string
		Matches bool
	}{
		// Manual tests.
		{`Prompt: test`, true},
		{`Parameters: "test"`, true},
		{`Negative_Prompt: x`, true},
		{`sampler: abcd`, true},
		{`seed: 123456`, true},
		{`CFG_Scale: 5`, true},
		{`midjourney: 1.0`, true},

		// Sample image 1 (no match).
		{"Filename: 1000063323.jpg", false},
		{"SceneCaptureType: 0", false},
		{`DateTimeDigitized: "2025:10:27 09:59:45"`, false},
		{`ExposureBiasValue: "0/10"`, false},
		{"ColorSpace: 1", false},
		{"Saturation: 0", false},
		{`DigitalZoomRatio: "16/16"`, false},
		{`XMP: XMP: crs:ToneCurvePV2012 rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012 crs:ToneCurvePV2012Red rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012Red crs:ToneCurvePV2012Green rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012Green crs:ToneCurvePV2012Blue rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012Blue crs:PointColors rdf:Seq rdf:li-1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000, -1.000000/rdf:li /rdf:Seq /crs:PointColors crs:ColorVariance rdf:Seq rdf:li-50.000000/rdf:li /rdf:Seq /crs:ColorVariance crs:Look crs:SortName rdf:Alt B&amp;W 01/rdf:li /rdf:Alt /crs:SortName crs:Group rdf:Alt B&amp;W/rdf:li /rdf:Alt /crs:Group crs:Parameters crs:ToneCurvePV2012 rdf:Seq rdf:li0, 0/rdf:li rdf:li17, 17/rdf:li rdf:li34, 36/rdf:li rdf:li51, 55/rdf:li rdf:li68, 75/rdf:li rdf:li85, 96/rdf:li rdf:li102, 116/rdf:li rdf:li119, 136/rdf:li rdf:li136, 154/rdf:li rdf:li153, 171/rdf:li rdf:li170, 187/rdf:li rdf:li187, 202/rdf:li rdf:li204, 216/rdf:li rdf:li221, 230/rdf:li rdf:li238, 242/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012 crs:ToneCurvePV2012Red rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012Red crs:ToneCurvePV2012Green rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012Green crs:ToneCurvePV2012Blue rdf:Seq rdf:li0, 0/rdf:li rdf:li255, 255/rdf:li /rdf:Seq /crs:ToneCurvePV2012Blue /rdf:Description /crs:Parameters /rdf:Description /crs:Look xmpMM:History rdf:Seq /rdf:Seq /xmpMM:History /rdf:Description /rdf:RDF /x:xmpmeta`, false},
		{`FNumber: "45/10"`, false},
		{`CustomRendered: 0`, false},
		{`Sharpness: 0`, false},
		{`FocalLength: "550/10"`, false},
		{`Flash: 16`, false},
		{`DateTime: "2025:10:27 14:50:16"`, false},
		{`FocalLengthIn35mmFilm: 82`, false},
		{`MaxApertureValue: "1110/256"`, false},
		{`ExposureMode: 0`, false},
		{`ApertureValue: "433985/100000"`, false},
		{`SceneType: ""`, false},
		{`Make: "SONY"`, false},
		{`Model: "ILCE-5100"`, false},
		{`DateTimeOriginal: "2025:10:27 09:59:45"`, false},
		{`ExposureProgram: 2`, false},
		{`ExifVersion: "0231"`, false},
		{`ExifIFDPointer: 196`, false},
		{`FileSource: ""`, false},
		{`MeteringMode: 5`, false},
		{`XResolution: "300/1"`, false},
		{`YResolution: "300/1"`, false},
		{`BrightnessValue: "1464/2560"`, false},
		{`LightSource: 0`, false},
		{`Software: "Adobe Lightroom 10.5.4 (Android)"`, false},
		{`LensModel: "E 55-210mm F4.5-6.3 OSS"`, false},
		{`ResolutionUnit: 2`, false},
		{`WhiteBalance: 0`, false},
		{`Contrast: 0`, false},
		{`ISOSpeedRatings: 3200`, false},
		{`ShutterSpeedValue: "6643856/1000000"`, false},
		{`ExposureTime: "1/100"`, false},

		// Sample image 2 (no match).
		{`Filename: coffeeSmall.jpg`, false},
		{`MeteringMode: 2`, false},
		{`ImageUniqueID: "L12XLLD01VM"`, false},
		{`Flash: 0`, false},
		{`SamplesPerPixel: 3`, false},
		{`ResolutionUnit: 2`, false},
		{`ApertureValue: "252/100"`, false},
		{`Model: "SM-G970F"`, false},
		{`ShutterSpeedValue: "504/100"`, false},
		{`ColorSpace: 1`, false},
		{`XResolution: "720000/10000"`, false},
		{`BrightnessValue: "20/100"`, false},
		{`FNumber: "240/100"`, false},
		{`ExposureProgram: 2`, false},
		{`ImageLength: 3024`, false},
		{`Orientation: 1`, false},
		{`BitsPerSample: [8,8,8]`, false},
		{`YCbCrPositioning: 1`, false},
		{`ExposureBiasValue: "0/100"`, false},
		{`XMP: xmpMM:History rdf:Seq /rdf:Seq /xmpMM:History /rdf:Description /rdf:RDF /x:xmpmeta`, false},
		{`DateTimeDigitized: "2025:10:23 14:33:09"`, false},
		{`MaxApertureValue: "252/100"`, false},
		{`PhotometricInterpretation: 2`, false},
		{`DateTimeOriginal: "2025:10:23 14:33:09"`, false},
		{`YResolution: "720000/10000"`, false},
		{`SceneCaptureType: 0`, false},
		{`ISOSpeedRatings: 400`, false},
		{`Software: "Adobe Photoshop CC 2015 (Windows)"`, false},
		{`DateTime: "2025:10:27 09:03:51"`, false},
		{`ExifIFDPointer: 288`, false},
		{`WhiteBalance: 0`, false},
		{`PixelXDimension: 855`, false},
		{`ImageWidth: 4032`, false},
		{`ExifVersion: "0220"`, false},
		{`PixelYDimension: 1280`, false},
		{`ThumbJPEGInterchangeFormat: 838`, false},
		{`FocalLength: "432/100"`, false},
		{`Make: "samsung"`, false},
		{`ThumbJPEGInterchangeFormatLength: 3819`, false},
		{`ExposureMode: 0`, false},
		{`FocalLengthIn35mmFilm: 26`, false},
		{`DigitalZoomRatio: "156/100"`, false},
		{`ExposureTime: "1/33"`, false},

		// Sample image 3: no match.
		{`Filename: PSX_20251026_143220.jpg`, false},
		{`ExifIFDPointer: 156`, false},
		{`XResolution: "72/1"`, false},
		{`ResolutionUnit: 2`, false},
		{`Software: "Adobe Photoshop Express (Android)"`, false},
		{`DateTime: "2025:10:26 14:32:20"`, false},
		{`XMP: xmpMM:History rdf:Seq /rdf:Seq /xmpMM:History /rdf:Description /rdf:RDF /x:xmpmeta`, false},
		{`ExifVersion: "0231"`, false},
		{`ColorSpace: 1`, false},
		{`YResolution: "72/1"`, false},

		// Matched image 1 (Stable Diffusion)
		{`Filename: 00018-298495402.png`, false},
		{`parameters: Santa Claus on a motorcycle. Steps: 20, Sampler: DPM++ 2M, Schedule type: Karras, CFG scale: 7, Seed: 298495402, Size: 512x512, Model hash: 3e71f329d8, Model: photorealisticAllPurpose_v30, Version: f1.0.0v2-v1.10.1RC-latest-2454-g9086b215`, true},
	}

	for i, test := range cases {
		var (
			parts  = strings.SplitN(test.Str, ":", 2)
			key    = strings.TrimSpace(parts[0])
			value  = strings.TrimSpace(parts[1])
			actual = containsAIMetadata(key) || containsAIMetadata(value)
			expect = test.Matches
		)
		if actual != expect {
			t.Errorf("Case #%d: `%s` expected match to be %v but it was %v", i, test.Str, expect, actual)
		}
	}
}
