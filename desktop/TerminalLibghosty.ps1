Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[Windows.Forms.Application]::EnableVisualStyles()
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
function Field($caption,$y){$a=New-Object Windows.Forms.Label;$a.Text=$caption;$a.Location="24,$y";$a.Size='420,22';$a.ForeColor='Silver';$pnl.Controls.Add($a);$b=New-Object Windows.Forms.TextBox;$b.Location="24,$($y+25)";$b.Size='420,30';$b.Font=New-Object Drawing.Font('Segoe UI',11);$b.BackColor=[Drawing.Color]::FromArgb(15,19,26);$b.ForeColor='White';$pnl.Controls.Add($b);return $b}
$n=Field 'CONNECTION NAME' 20;$t=Field 'SSH ADDRESS (user@host[:port])' 95;$po=Field 'PORT OVERRIDE (OPTIONAL)' 170;$k=Field 'PRIVATE KEY FILE (OPTIONAL)' 245
$st=New-Object Windows.Forms.Label;$st.Location='28,570';$st.Size='920,35';$st.Anchor='Bottom,Left,Right';$st.ForeColor='LightGreen';$st.Text='Ready · SSH host-key verification enabled';$f.Controls.Add($st)
function Button($text,$x,$y,$w,$action,$parent=$f){$b=New-Object Windows.Forms.Button;$b.Text=$text;$b.Location="$x,$y";$b.Size="$w,42";$b.FlatStyle='Flat';$b.FlatAppearance.BorderSize=0;$b.BackColor=[Drawing.Color]::FromArgb(48,112,210);$b.ForeColor='White';$b.Font=New-Object Drawing.Font('Segoe UI',10,[Drawing.FontStyle]::Bold);$b.Add_Click($action);$parent.Controls.Add($b)}
Button 'SAVE CONNECTION' 504 490 190 { $v=$t.Text.Trim();if($v -notmatch '^[A-Za-z0-9._-]+@(?:\[[0-9a-fA-F:]+\]|[A-Za-z0-9.-]+)(?::[0-9]{1,5})?$'){[Windows.Forms.MessageBox]::Show('Enter user@host[:port]');return};$num=0;if($po.Text.Trim() -and (-not [int]::TryParse($po.Text,[ref]$num) -or $num -lt 1 -or $num -gt 65535)){[Windows.Forms.MessageBox]::Show('Port must be 1-65535');return};$script:profiles+=@([pscustomobject]@{Name=$(if($n.Text.Trim()){$n.Text.Trim()}else{$v});Target=$v;Port=$num;Identity=$k.Text.Trim()});SaveProfiles;RefreshList;$l.SelectedIndex=$l.Items.Count-1;$st.Text='Connection saved.'} $pnl
Button 'CONNECT  →' 712 490 190 {if($l.SelectedIndex -lt 0){$st.Text='Select a connection first.';return};$q=$script:profiles[$l.SelectedIndex];$args=@('-tt','-o','RequestTTY=force','-o','StrictHostKeyChecking=yes');if($q.Identity){$args+=@('-i',$q.Identity)};if($q.Port -gt 0){$args+=@('-p',[string]$q.Port)};$args+=@('--',$q.Target);try{Start-Process wt.exe -ArgumentList (@('new-tab','--title',$q.Name,'ssh')+$args);$st.Text="Connecting to $($q.Target)..."}catch{$st.Text=$_.Exception.Message}} $pnl
Button 'DELETE' 28 490 130 {if($l.SelectedIndex -ge 0){$i=$l.SelectedIndex;$script:profiles=@($script:profiles|Where-Object {$_ -ne $script:profiles[$i]});SaveProfiles;RefreshList;$st.Text='Connection deleted.'}}
$l.Add_SelectedIndexChanged({if($l.SelectedIndex -ge 0){$q=$script:profiles[$l.SelectedIndex];$n.Text=$q.Name;$t.Text=$q.Target;$po.Text=if($q.Port){[string]$q.Port}else{''};$k.Text=$q.Identity}})
RefreshList
[void]$f.ShowDialog()
