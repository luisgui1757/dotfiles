package installer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type fontFace struct {
	File, Family, PostScript, Style string
	Bold, Italic                    bool
}

// Names are the SFNT name tables in checksum-pinned Hack.zip 3.5.1. No registry
// search by a friendly-name prefix grants ownership of any font.
func hackFontFaces() []fontFace {
	result := []fontFace{}
	for _, family := range []struct{ file, name, post string }{{"HackNerdFont", "Hack Nerd Font", "HackNF"}, {"HackNerdFontMono", "Hack Nerd Font Mono", "HackNFM"}, {"HackNerdFontPropo", "Hack Nerd Font Propo", "HackNFP"}} {
		for _, style := range []string{"Regular", "Bold", "Italic", "BoldItalic"} {
			result = append(result, fontFace{family.file + "-" + style + ".ttf", family.name, family.post + "-" + style, style, strings.Contains(style, "Bold"), strings.Contains(style, "Italic")})
		}
	}
	return result
}

type fontNativeFace struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Hash         string `json:"hash"`
	Glyph        bool   `json:"glyph"`
	Registration string `json:"registration"`
}

func runIntegrationQuery(ctx context.Context, command nativeCommand) ([]byte, error) {
	if command.Operation != "" {
		return nil, errors.New("read-only integration query cannot carry operation identity")
	}
	if err := command.validate(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, command.Program, command.Arguments...)
	cmd.Stdin = strings.NewReader(string(command.Input))
	// Only stdout carries the inspection protocol. Encoded PowerShell commands
	// can emit successful module-loading progress as CLIXML on stderr.
	var output, diagnostic nativeOutput
	cmd.Stdout, cmd.Stderr = &output, &diagnostic
	err := cmd.Run()
	if output.exceeded || diagnostic.exceeded || output.data.Len()+diagnostic.data.Len() > 1<<20 {
		err = errors.Join(err, errors.New("native integration inspection exceeds 1 MiB"))
	}
	if err != nil {
		return output.data.Bytes(), fmt.Errorf("native integration inspection failed: %w%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes(), err
}
func fontFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16<<20+1))
	if err != nil || len(data) > 16<<20 {
		return "", errors.Join(errors.New("font exceeds inspection bound"), err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
func (d *FontDriver) query(ctx context.Context, paths []string) ([]fontNativeFace, error) {
	faces := hackFontFaces()
	result := make([]fontNativeFace, len(faces))
	run := d.Query
	if run == nil {
		run = runIntegrationQuery
	}
	switch d.Target.OS {
	case "linux":
		if !filepath.IsAbs(d.FontMatch) {
			return nil, errors.New("fontconfig fc-match is unavailable")
		}
		for i, face := range faces {
			style := strings.ReplaceAll(face.Style, "BoldItalic", "Bold Italic")
			output, err := run(ctx, nativeCommand{Program: d.FontMatch, Arguments: []string{"-f", "%{postscriptname}\n%{file}\n%{charset}\n", face.Family + ":style=" + style}})
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(string(output)) == "" {
				continue
			}
			parts := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(parts) != 3 {
				return nil, errors.New("unexpected bounded fontconfig match")
			}
			result[i].Name, result[i].Path = parts[0], parts[1]
			result[i].Glyph = fontCharsetContains(parts[2], 0xf120)
		}
	case "darwin":
		names := []string{}
		for _, face := range faces {
			names = append(names, face.PostScript)
		}
		data, _ := json.Marshal(names)
		script := `ObjC.import('CoreText'); var names=` + string(data) + `; var result=names.map(function(name){ var f=$.CTFontCreateWithName($(name),12,null); var u=$.CTFontCopyAttribute(f,$.kCTFontURLAttribute); return {name:ObjC.unwrap(ObjC.castRefToObject($.CTFontCopyPostScriptName(f))),path:u?ObjC.unwrap(u.path):'',glyph:!!$.CFCharacterSetIsLongCharacterMember($.CTFontCopyCharacterSet(f),0xf120)}; }); JSON.stringify(result);`
		output, err := run(ctx, nativeCommand{Program: "/usr/bin/osascript", Arguments: []string{"-l", "JavaScript", "-e", script}})
		if err != nil {
			return nil, err
		}
		if err := Decode(output, &result); err != nil {
			return nil, err
		}
	case "windows":
		data, _ := json.Marshal(struct {
			Faces []fontFace `json:"faces"`
			Paths []string   `json:"paths"`
		}{faces, paths})
		output, err := run(ctx, nativeCommand{Program: d.PowerShell, Arguments: windowsVendorArguments(windowsFontPrelude + windowsFontQuery), Input: data})
		if err != nil {
			return nil, fmt.Errorf("inspect current-user fonts: %w: %s", err, nativeErrorDetail(err, output))
		}
		if err := Decode(output, &result); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported font platform")
	}
	if len(result) != len(faces) {
		return nil, errors.New("font inspection returned wrong face count")
	}
	for i := range result {
		if d.Target.OS != "windows" && result[i].Name == faces[i].PostScript && filepath.IsAbs(result[i].Path) {
			hash, err := fontFileHash(result[i].Path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			result[i].Hash = hash
		}
		if len(result[i].Path) > 32768 || len(result[i].Registration) > 32768 || len(result[i].Name) > 256 {
			return nil, errors.New("font identity exceeds bound")
		}
	}
	return result, nil
}
func fontCharsetContains(value string, wanted uint64) bool {
	for _, item := range strings.Fields(value) {
		var first, last uint64
		parts := strings.Split(item, "-")
		if len(parts) > 2 {
			continue
		}
		if _, err := fmt.Sscanf(parts[0], "%x", &first); err != nil {
			continue
		}
		last = first
		if len(parts) == 2 {
			if _, err := fmt.Sscanf(parts[1], "%x", &last); err != nil {
				continue
			}
		}
		if wanted >= first && wanted <= last {
			return true
		}
	}
	return false
}

const windowsFontPrelude = `
$ErrorActionPreference='Stop'
[Console]::InputEncoding=[Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
$inputData=[Console]::In.ReadToEnd() | ConvertFrom-Json
Add-Type -TypeDefinition @'
using System;
using System.Text;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
public static class DotfilesFont {
	[DllImport("gdi32.dll",CharSet=CharSet.Unicode)] static extern IntPtr CreateFontW(int h,int w,int e,int o,int weight,uint italic,uint underline,uint strike,uint charset,uint output,uint clip,uint quality,uint pitch,string face);
	[DllImport("gdi32.dll")] static extern IntPtr CreateCompatibleDC(IntPtr dc);
	[DllImport("gdi32.dll")] static extern IntPtr SelectObject(IntPtr dc,IntPtr obj);
	[DllImport("gdi32.dll")] static extern bool DeleteObject(IntPtr obj);
	[DllImport("gdi32.dll")] static extern bool DeleteDC(IntPtr dc);
	[DllImport("gdi32.dll")] static extern uint GetFontData(IntPtr dc,uint table,uint offset,byte[] data,uint count);
	[DllImport("gdi32.dll",CharSet=CharSet.Unicode)] static extern int GetTextFaceW(IntPtr dc,int count,StringBuilder name);
	[DllImport("gdi32.dll",CharSet=CharSet.Unicode)] static extern uint GetGlyphIndicesW(IntPtr dc,string text,int count,ushort[] glyphs,uint flags);
	[DllImport("gdi32.dll",CharSet=CharSet.Unicode)] public static extern int AddFontResourceExW(string path,uint flags,IntPtr reserved);
	[DllImport("gdi32.dll",CharSet=CharSet.Unicode)] public static extern bool RemoveFontResourceExW(string path,uint flags,IntPtr reserved);
	[DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern IntPtr SendMessageTimeoutW(IntPtr window,uint message,IntPtr w,IntPtr l,uint flags,uint timeout,out IntPtr result);
	static int U16(byte[] data,int p) {if(p<0||p+2>data.Length)throw new Exception("Invalid native font table");return (data[p]<<8)|data[p+1];}
	static int U32(byte[] data,int p) {return checked(U16(data,p)*65536+U16(data,p+2));}
	static string PostScript(byte[] data) {
		int tables=U16(data,4);if(tables>256)throw new Exception("Native font table count exceeds bound");
		for(int i=0;i<tables;i++){int p=12+i*16;if(p+16>data.Length)throw new Exception("Invalid native font directory");if(data[p]!=110||data[p+1]!=97||data[p+2]!=109||data[p+3]!=101)continue;
			int start=U32(data,p+8),count=U16(data,start+2),storage=U16(data,start+4);if(count>4096)throw new Exception("Native font name count exceeds bound");
			for(int j=0;j<count;j++){int q=start+6+12*j;if(U16(data,q)!=3||U16(data,q+6)!=6)continue;int length=U16(data,q+8),offset=start+storage+U16(data,q+10);if(length>512||offset<0||offset+length>data.Length)throw new Exception("Invalid native font name");return Encoding.BigEndianUnicode.GetString(data,offset,length);}
		}return "";
	}
	public static string[] Probe(string family,bool bold,bool italic) {
		IntPtr dc=CreateCompatibleDC(IntPtr.Zero); IntPtr font=CreateFontW(-24,0,0,0,bold?700:400,italic?1u:0u,0,0,1,0,0,0,0,family); if(dc==IntPtr.Zero||font==IntPtr.Zero)throw new Exception("Cannot create native font consumer");
		IntPtr old=SelectObject(dc,font);
		try {
			StringBuilder name=new StringBuilder(256); if(GetTextFaceW(dc,256,name)==0)throw new Exception("Cannot inspect selected font");
			uint count=GetFontData(dc,0,0,null,0); if(count==0xffffffff||count>16777216)return new[]{name.ToString(),"","false",""};
			byte[] data=new byte[count]; if(GetFontData(dc,0,0,data,count)!=count)throw new Exception("Cannot read selected font bytes");
			ushort[] glyph=new ushort[1]; bool present=GetGlyphIndicesW(dc,"\uf120",1,glyph,1)!=0xffffffff&&glyph[0]!=0xffff&&glyph[0]!=0;
			using(SHA256 sha=SHA256.Create())return new[]{name.ToString(),BitConverter.ToString(sha.ComputeHash(data)).Replace("-","").ToLowerInvariant(),present?"true":"false",PostScript(data)};
		} finally {SelectObject(dc,old);DeleteObject(font);DeleteDC(dc);}
	}
}
'@
$fontKey='Software\Microsoft\Windows NT\CurrentVersion\Fonts'
function Get-Registration([string]$name) {
	$key=[Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($fontKey)
	if (!$key) { return '' }
	try { if ($key.GetValueNames() -cnotcontains $name) { return '' }; if ($key.GetValueKind($name) -ne [Microsoft.Win32.RegistryValueKind]::String) { throw 'Font registration has an unexpected native type' }; return [string]$key.GetValue($name,$null,[Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) } finally {$key.Dispose()}
}
`
const windowsFontQuery = `
$result=@();for($i=0;$i -lt $inputData.faces.Count;$i++) {
	$face=$inputData.faces[$i];$probe=[DotfilesFont]::Probe($face.Family,$face.Bold,$face.Italic)
	$name='';if($probe[0] -ceq $face.Family){$name=$probe[3]}
	$result+=@{name=$name;path='';hash=$probe[1];glyph=($probe[2] -eq 'true');registration=(Get-Registration ('Dotfiles '+$face.File+' (TrueType)'))}
}
ConvertTo-Json -InputObject @($result) -Compress
`
const windowsFontChange = `
# Revalidate all keys before the first change; exact old values are approval data.
for($i=0;$i -lt $inputData.faces.Count;$i++) {
	$name='Dotfiles '+$inputData.faces[$i].File+' (TrueType)'
	if ((Get-Registration $name) -cne $inputData.before[$i]) {throw 'Font registration changed after approval'}
	if ($inputData.before[$i] -ne '' -and $inputData.before[$i] -cne $inputData.paths[$i]) {throw 'Foreign font registration is retained'}
}
$key=[Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($fontKey)
try {
	for($i=0;$i -lt $inputData.faces.Count;$i++) {
		$name='Dotfiles '+$inputData.faces[$i].File+' (TrueType)';$path=$inputData.paths[$i]
		if ((Get-Registration $name) -cne $inputData.before[$i]) {throw 'Font registration changed during native publication'}
		if ($inputData.install) {
			if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -cne $inputData.hashes[$i]) {throw 'Owned font file changed before registration'}
			$key.SetValue($name,$path,[Microsoft.Win32.RegistryValueKind]::String)
			if ([DotfilesFont]::AddFontResourceExW($path,0,[IntPtr]::Zero) -eq 0) {throw 'Native font activation failed; registration is saved for recovery'}
		} elseif ($inputData.before[$i] -ne '') {
			if ($inputData.hashes[$i] -eq 'absent') {if (Test-Path -LiteralPath $path) {throw 'Font file appeared after removal approval'}} elseif ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -cne $inputData.hashes[$i]) {throw 'Changed font file is retained'}
			if ($inputData.hashes[$i] -ne 'absent' -and ![DotfilesFont]::RemoveFontResourceExW($path,0,[IntPtr]::Zero)) {throw 'Native font is still loaded or could not be released; close font consumers and resume, or restart before retrying'}
			$key.DeleteValue($name,$false)
		}
	}
} finally {$key.Dispose()}
[IntPtr]$result=[IntPtr]::Zero
[void][DotfilesFont]::SendMessageTimeoutW([IntPtr]0xffff,0x001d,[IntPtr]::Zero,[IntPtr]::Zero,2,2000,[ref]$result)
`
