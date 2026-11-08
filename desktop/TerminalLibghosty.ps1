Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[Windows.Forms.Application]::EnableVisualStyles()
$script:logDir=Join-Path $PSScriptRoot 'logs'
New-Item -ItemType Directory -Force $script:logDir | Out-Null
$script:guiLog=Join-Path $script:logDir 'gui.log'
function Write-Log([string]$Message,[string]$Level='INFO') {
 $line='[{0}] [{1}] {2}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff'),$Level,$Message
 Add-Content -LiteralPath $script:guiLog -Value $line -Encoding UTF8
 Write-Host $line
}
Write-Log 'GUI startup.'
try { Start-Transcript -LiteralPath (Join-Path $script:logDir 'gui-transcript.log') -Append -ErrorAction Stop | Out-Null } catch { Write-Log "Could not start transcript: $($_.Exception.Message)" 'WARN' }
Add-Type @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class TerminalLibghostyWin32 {
 [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr parent, EnumProc callback, IntPtr lParam);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern int GetClassName(IntPtr hwnd, StringBuilder name, int max);
 [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hwnd, uint msg, IntPtr wParam, IntPtr lParam);
 delegate bool EnumProc(IntPtr hwnd, IntPtr lParam);
 public static IntPtr FindChild(IntPtr parent, string className) {
  IntPtr found=IntPtr.Zero;
  EnumChildWindows(parent, (h,l) => { var b=new StringBuilder(256); GetClassName(h,b,b.Capacity); if(b.ToString()==className){found=h;return false;} return true; }, IntPtr.Zero);
  return found;
 }
}
'@
$store=Join-Path $env:APPDATA 'TerminalLibghosty\connections.json'
function LoadProfiles { if(Test-Path $store){try{return @(Get-Content $store -Raw|ConvertFrom-Json)}catch{}}; @() }
function SaveProfiles { New-Item -ItemType Directory -Force (Split-Path $store)|Out-Null; $script:profiles|ConvertTo-Json -Depth 4|Set-Content -Encoding UTF8 $store }
$script:profiles=@(LoadProfiles)
$f=New-Object Windows.Forms.Form; $f.Text='Terminal Libghosty - SSH Manager'; $f.Size='1000,680'; $f.MinimumSize='850,560'; $f.StartPosition='CenterScreen'; $f.BackColor=[Drawing.Color]::FromArgb(18,22,30); $f.ForeColor='White'
$h=New-Object Windows.Forms.Label; $h.Text='TERMINAL LIBGHOSTY'; $h.Font=New-Object Drawing.Font('Segoe UI',20,[Drawing.FontStyle]::Bold); $h.Location='28,20'; $h.Size='500,40'; $f.Controls.Add($h)
$s=New-Object Windows.Forms.Label; $s.Text='SSH CONNECTION MANAGER'; $s.Location='31,65'; $s.Size='500,25'; $s.ForeColor='Silver'; $f.Controls.Add($s)
$l=New-Object Windows.Forms.ListBox; $l.Location='28,112'; $l.Size='420,440'; $l.Anchor='Top,Bottom,Left'; $l.BackColor=[Drawing.Color]::FromArgb(27,33,44); $l.ForeColor='White'; $l.Font=New-Object Drawing.Font('Segoe UI',11); $f.Controls.Add($l)
function RefreshList {$l.Items.Clear();foreach($p in $script:profiles){[void]$l.Items.Add("$($p.Name)  ·  $($p.Target)")}}
$pnl=New-Object Windows.Forms.Panel; $pnl.Location='480,112'; $pnl.Size='475,440'; $pnl.Anchor='Top,Bottom,Left,Right'; $pnl.BackColor=[Drawing.Color]::FromArgb(27,33,44); $f.Controls.Add($pnl)
$termHost=New-Object Windows.Forms.Panel; $termHost.Location='10,50'; $termHost.Size='455,380'; $termHost.Anchor='Top,Bottom,Left,Right'; $termHost.BackColor=[Drawing.Color]::FromArgb(15,19,26); $termHost.Visible=$false; $pnl.Controls.Add($termHost)
function Field($caption,$y){$a=New-Object Windows.Forms.Label;$a.Text=$caption;$a.Location="24,$y";$a.Size='420,22';$a.ForeColor='Silver';$pnl.Controls.Add($a);$b=New-Object Windows.Forms.TextBox;$b.Location="24,$($y+25)";$b.Size='420,30';$b.Font=New-Object Drawing.Font('Segoe UI',11);$b.BackColor=[Drawing.Color]::FromArgb(15,19,26);$b.ForeColor='White';$pnl.Controls.Add($b);return $b}
$n=Field 'CONNECTION NAME' 20;$t=Field 'SSH ADDRESS (user@host[:port])' 95;$po=Field 'PORT OVERRIDE (OPTIONAL)' 170;$k=Field 'PRIVATE KEY FILE (OPTIONAL)' 245
$st=New-Object Windows.Forms.Label;$st.Location='28,570';$st.Size='920,35';$st.Anchor='Bottom,Left,Right';$st.ForeColor='LightGreen';$st.Text='Ready · SSH host-key verification enabled';$f.Controls.Add($st)
function Button($text,$x,$y,$w,$action,$parent=$f){$b=New-Object Windows.Forms.Button;$b.Text=$text;$b.Location="$x,$y";$b.Size="$w,42";$b.FlatStyle='Flat';$b.FlatAppearance.BorderSize=0;$b.BackColor=[Drawing.Color]::FromArgb(48,112,210);$b.ForeColor='White';$b.Font=New-Object Drawing.Font('Segoe UI',10,[Drawing.FontStyle]::Bold);$b.Add_Click($action);$parent.Controls.Add($b);return $b}
Button 'SAVE CONNECTION' 24 350 190 { $v=$t.Text.Trim();if($v -notmatch '^[A-Za-z0-9._-]+@(?:\[[0-9a-fA-F:]+\]|[A-Za-z0-9.-]+)(?::[0-9]{1,5})?$'){[Windows.Forms.MessageBox]::Show('Enter user@host[:port]');return};$num=0;if($po.Text.Trim() -and (-not [int]::TryParse($po.Text,[ref]$num) -or $num -lt 1 -or $num -gt 65535)){[Windows.Forms.MessageBox]::Show('Port must be 1-65535');return};$script:profiles+=@([pscustomobject]@{Name=$(if($n.Text.Trim()){$n.Text.Trim()}else{$v});Target=$v;Port=$num;Identity=$k.Text.Trim()});SaveProfiles;RefreshList;$l.SelectedIndex=$l.Items.Count-1;$st.Text='Connection saved.'} $pnl
$script:hostProcess=$null
$script:hostHwnd=[IntPtr]::Zero
function StopEmbeddedHost {
 if($script:hostHwnd -ne [IntPtr]::Zero){[TerminalLibghostyWin32]::PostMessage($script:hostHwnd,0x0010,[IntPtr]::Zero,[IntPtr]::Zero)|Out-Null;$script:hostHwnd=[IntPtr]::Zero}
 if($script:hostProcess){try{Wait-Process -Id $script:hostProcess.Id -Timeout 3 -ErrorAction Stop}catch{};try{if(!$script:hostProcess.HasExited){Stop-Process -Id $script:hostProcess.Id -Force}}catch{}}
 $script:hostProcess=$null
}
$script:profileControls=@($pnl.Controls | Where-Object { $_ -ne $termHost })
$disconnect=Button '← BACK / DISCONNECT' 10 5 210 {StopEmbeddedHost;$termHost.Visible=$false;foreach($c in $script:profileControls){$c.Visible=$true};$disconnect.Visible=$false;$connectButton.Visible=$true;$deleteButton.Visible=$true;$st.Text='Disconnected.'} $pnl
$disconnect.Visible=$false
$connectButton=Button 'CONNECT  →' 232 350 190 {
 Write-Log 'Connect button clicked.'
 if($l.SelectedIndex -lt 0){$st.Text='Select a connection first.';return}
 $q=$script:profiles[$l.SelectedIndex]
 Write-Log ("Selected profile: target={0}; port={1}; identityConfigured={2}" -f $q.Target,$q.Port,[bool]$q.Identity)
 $root=Split-Path $PSScriptRoot -Parent
 $native=Join-Path $root 'build\native-local'
 $hostExe=Join-Path $root 'dist\terminal-host.exe'
 if(!(Test-Path (Join-Path $native 'terminal_libghosty_session.dll')) -or !(Test-Path (Join-Path $native 'ghostty-vt.dll'))){Write-Log "Native DLL check failed: $native" 'ERROR';$st.Text='Native Ghostty DLLs not found. Build native\ first.';return}
 $hostSrc=Join-Path $root 'cmd\terminal-host\main_windows.go'
 if(!(Test-Path $hostExe) -or ((Get-Item $hostSrc).LastWriteTime -gt (Get-Item $hostExe).LastWriteTime)){
  try{Write-Log 'terminal-host.exe missing or outdated; building it now.';New-Item -ItemType Directory -Force (Split-Path $hostExe)|Out-Null;$oldDir=Get-Location;Set-Location $root;$env:TERMINAL_LIBGHOSTY_NATIVE_DIR=$native;& go build -ldflags='-H=windowsgui' -o $hostExe ./cmd/terminal-host;if($LASTEXITCODE -ne 0){throw "Go terminal host build failed (exit=$LASTEXITCODE)"};Set-Location $oldDir;Write-Log 'terminal-host.exe build succeeded.'}catch{Write-Log "Host build failed: $($_.Exception.Message)" 'ERROR';$st.Text=$_.Exception.Message;return}
 }
 $env:TERMINAL_LIBGHOSTY_NATIVE_DIR=$native
 $parentHandle='0x{0:X}' -f $termHost.Handle.ToInt64()
 $argLine='--ssh "{0}" --parent {1}' -f $q.Target,$parentHandle
 if($q.Port -gt 0){$argLine+=' --port '+[string]$q.Port}
 if($q.Identity){$argLine+=' --identity "'+$q.Identity+'"'}
 try{
  $hostStdout=Join-Path $script:logDir 'host.stdout.log';$hostStderr=Join-Path $script:logDir 'host.stderr.log'
  Write-Log ("Starting host: exe={0}; parent={1}; args={2}" -f $hostExe,$parentHandle,$argLine)
  foreach($c in $script:profileControls){$c.Visible=$false}
  $termHost.Visible=$true;$termHost.BringToFront();$connectButton.Visible=$false;$deleteButton.Visible=$false;$disconnect.Visible=$true;$disconnect.BringToFront()
  $script:hostProcess=Start-Process -FilePath $hostExe -ArgumentList $argLine -PassThru -WindowStyle Hidden -RedirectStandardOutput $hostStdout -RedirectStandardError $hostStderr
  Write-Log "Host process started: pid=$($script:hostProcess.Id); stdout=$hostStdout; stderr=$hostStderr"
  for($i=0;$i -lt 40 -and !$script:hostProcess.HasExited;$i++){
   [Windows.Forms.Application]::DoEvents()
   $script:hostHwnd=[TerminalLibghostyWin32]::FindChild($termHost.Handle,'TerminalLibghostyEmbedded')
   if($script:hostHwnd -ne [IntPtr]::Zero){Write-Log ("Embedded child window found: hwnd=0x{0:X}; waitMs={1}" -f $script:hostHwnd.ToInt64(),($i*50));break}
   Start-Sleep -Milliseconds 50
  }
  if($script:hostHwnd -eq [IntPtr]::Zero){$exitInfo='still running';try{if($script:hostProcess.HasExited){$exitInfo="exit code $($script:hostProcess.ExitCode)"}}catch{};$errTail='';if(Test-Path $hostStderr){$errTail=(Get-Content -LiteralPath $hostStderr -Tail 30 -ErrorAction SilentlyContinue) -join ' | '};Write-Log "Embedded window missing; host $exitInfo; stderr=$errTail" 'ERROR';throw "Embedded terminal child window failed to initialize (host $exitInfo). See $hostStderr"}
  Write-Log 'Connect initialization completed; SSH terminal child window is present.'
  $st.Text="Embedded SSH terminal starting: $($q.Target)"
 }catch{Write-Log "Connect failed: $($_.Exception.Message)" 'ERROR';StopEmbeddedHost;$termHost.Visible=$false;foreach($c in $script:profileControls){$c.Visible=$true};$connectButton.Visible=$true;$deleteButton.Visible=$true;$disconnect.Visible=$false;$st.Text=$_.Exception.Message}
} $pnl
$f.Add_FormClosing({StopEmbeddedHost})
$deleteButton=New-Object Windows.Forms.Button
$deleteButton.Text='DELETE';$deleteButton.Location='28,490';$deleteButton.Size='130,42';$deleteButton.FlatStyle='Flat';$deleteButton.BackColor=[Drawing.Color]::FromArgb(48,112,210);$deleteButton.ForeColor='White';$pnl.Controls.Add($deleteButton)
$deleteButton.Add_Click({
 if ($l.SelectedIndex -ge 0) {
  $i=$l.SelectedIndex
  $next=@()
  for($j=0;$j -lt $script:profiles.Count;$j++){if($j -ne $i){$next+= $script:profiles[$j]}}
  $script:profiles=$next
  SaveProfiles
  RefreshList
  $st.Text="Connection deleted."
 }
})
$l.Add_SelectedIndexChanged({if($l.SelectedIndex -ge 0){$q=$script:profiles[$l.SelectedIndex];$n.Text=$q.Name;$t.Text=$q.Target;$po.Text=if($q.Port){[string]$q.Port}else{''};$k.Text=$q.Identity}})
RefreshList
[void]$f.ShowDialog()
Write-Log 'GUI closed.'
try { Stop-Transcript | Out-Null } catch {}
