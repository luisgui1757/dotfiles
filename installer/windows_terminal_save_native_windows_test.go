package installer

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// Pinned v1.25.2733.0: TabRowControl.xaml names NewTabButton; MainPage.xaml
// names SaveButton and its MainPage.cpp handler calls WriteSettingsToDisk.
// Exercise that handler through the owned process's UI, never by emulating JSON.
func windowsTerminalNativeSave(t *testing.T, process int, window int64, settings string) {
	t.Helper()
	program, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Fatal(err)
	}
	script := `$ErrorActionPreference='Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$ownedPID=` + strconv.Itoa(process) + `
$ownedWindow=[IntPtr]` + strconv.FormatInt(window, 10) + `
$settings=` + desktopPSQuote(settings) + `
$root=[Windows.Automation.AutomationElement]::FromHandle($ownedWindow)
if ($null -eq $root -or $root.Current.ProcessId -ne $ownedPID) { throw 'Terminal window does not belong to the fixture process' }
$deadline=[DateTime]::UtcNow.AddSeconds(20)
function Find-Control($scope,$condition,$description) {
	do {
		$element=$scope.FindFirst([Windows.Automation.TreeScope]::Descendants,$condition)
		if ($null -ne $element) { return $element }
		Start-Sleep -Milliseconds 100
	} while ([DateTime]::UtcNow -lt $deadline)
	throw ('Owned Terminal UI control unavailable: '+$description)
}
function By-Id([string]$value) { New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::AutomationIdProperty,$value) }
function Invoke-Control($element,[string]$description) {
	$pattern=$null
	if (-not $element.TryGetCurrentPattern([Windows.Automation.InvokePattern]::Pattern,[ref]$pattern)) { throw ('Owned Terminal control lacks InvokePattern: '+$description) }
	$pattern.Invoke()
}
$newTab=Find-Control $root (By-Id 'NewTabButton') 'NewTabButton'
$expand=$null
if (-not $newTab.TryGetCurrentPattern([Windows.Automation.ExpandCollapsePattern]::Pattern,[ref]$expand)) { throw 'NewTabButton lacks ExpandCollapsePattern' }
$expand.Expand()
# The pinned Settings flyout item has its localized name, not an AutomationId.
# Hosted acceptance uses English Windows; constrain popup lookup to this PID.
$conditions=[Windows.Automation.Condition[]]@(
	(New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::ProcessIdProperty,$ownedPID)),
	(New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::ControlTypeProperty,[Windows.Automation.ControlType]::MenuItem)),
	(New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::NameProperty,'Settings'))
)
$menu=Find-Control ([Windows.Automation.AutomationElement]::RootElement) ([Windows.Automation.AndCondition]::new($conditions)) 'Settings menu item in the owned PID'
Invoke-Control $menu 'Settings menu item'
$save=Find-Control $root (By-Id 'SaveButton') 'SaveButton'
$before=(Get-FileHash -LiteralPath $settings -Algorithm SHA256).Hash
$beforeWrite=(Get-Item -LiteralPath $settings).LastWriteTimeUtc.Ticks
Invoke-Control $save 'SaveButton'
do {
	$after=(Get-FileHash -LiteralPath $settings -Algorithm SHA256).Hash
	$afterWrite=(Get-Item -LiteralPath $settings).LastWriteTimeUtc.Ticks
	if ($afterWrite -ne $beforeWrite) { break }
	Start-Sleep -Milliseconds 100
} while ([DateTime]::UtcNow -lt $deadline)
if ($afterWrite -eq $beforeWrite) { throw 'Native Settings Save did not observably write settings.json' }
# Leave the witness shell as the only tab so its normal exit closes this window.
$tabCondition=New-Object Windows.Automation.PropertyCondition([Windows.Automation.AutomationElement]::ControlTypeProperty,[Windows.Automation.ControlType]::TabItem)
$tabs=$root.FindAll([Windows.Automation.TreeScope]::Descendants,$tabCondition)
$selected=@($tabs | Where-Object { $selection=$null; $_.TryGetCurrentPattern([Windows.Automation.SelectionItemPattern]::Pattern,[ref]$selection) -and $selection.Current.IsSelected })
if ($selected.Count -ne 1 -or $selected[0].Current.Name -ne 'Settings') { throw 'Cannot prove the selected tab is the owned Settings tab' }
$close=Find-Control $selected[0] (By-Id 'CloseButton') 'Settings tab CloseButton'
Invoke-Control $close 'Settings tab CloseButton'
@{before=$before.ToLowerInvariant();after=$after.ToLowerInvariant();beforeWrite=$beforeWrite;afterWrite=$afterWrite} | ConvertTo-Json -Compress
`
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	output, err := queryWindowsVendor(ctx, false, program, nil, windowsVendorArguments(script)...)
	if err != nil {
		t.Fatal("owned Terminal native Settings Save", err, "context:", ctx.Err(), string(output))
	}
	var result struct {
		Before, After           string
		BeforeWrite, AfterWrite int64
	}
	if err := json.Unmarshal(output, &result); err != nil || !operationID.MatchString(result.Before) || !operationID.MatchString(result.After) || result.BeforeWrite <= 0 || result.AfterWrite <= result.BeforeWrite {
		t.Fatal("native Settings Save lacks exact file-write evidence", string(output), err)
	}
	t.Logf("owned Terminal native Settings Save wrote settings: hash %s -> %s; UTC ticks %d -> %d", result.Before, result.After, result.BeforeWrite, result.AfterWrite)
}
