; The Windows installer.
;
; Three things make this a better answer than an archive:
;
;   it asks where to put the program, and remembers it;
;   it writes a configuration naming the two programs it needs, so that
;   "which yt-dlp is this" has an answer that can be read;
;   and it starts the application when it is done, which opens a browser.
;
; No administrator rights are needed. A per-user install goes under
; %LOCALAPPDATA%\Programs, which the user can write to, and the settings go
; under %APPDATA% beside it. An administrator may choose a machine-wide
; install instead; the wizard offers that, and it is not the default.
;
; Built by CI:
;   ISCC /DBundleDir=... /DAppVersion=4.10.3 packaging\windows\happy-downloader.iss
; BundleDir must contain the application, yt-dlp.exe, ffmpeg.exe, README.txt
; and happy-downloader.ico. The programs keep their plain names in the bundle,
; because the application looks for them beside itself.

#define AppName "Happy Downloader"
#define AppExeName "happy-downloader.exe"
#define AppPublisher "LeisureLinux"
#define AppURL "https://github.com/LeisureLinux/happy-downloader"

#ifndef BundleDir
  #error BundleDir was not defined
#endif
#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif


[Setup]
; A fixed id, so that a later version is recognised as an upgrade of this one
; rather than as a second program to install beside it.
AppId={{8E1F4C6A-2B7D-4F19-9A3E-5C0D7B21E4F8}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}/issues
AppUpdatesURL={#AppURL}/releases

DefaultDirName={autopf}\Happy Downloader
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
DisableDirPage=no

; Per-user by default: the application needs nothing a machine-wide install
; would give it, and requiring elevation to install a downloader is a good way
; to not get installed.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

OutputDir={#BundleDir}\..\dist
; The version is in the file name: an installer called only
; "happy-downloader" cannot be told apart from the one released before it.
OutputBaseFilename=happy-downloader_{#AppVersion}_windows-amd64_setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern

; x64 only. There is no Windows on ARM build: ffmpeg-static has no arm64
; Windows build, and an installer that cannot merge a video is not worth
; publishing.
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

; The product icon, in the wizard, in the executable, and beside every
; shortcut. Without SetupIconFile the installer wears the generic Inno Setup
; icon, and without an icon in the executable every shortcut wears the generic
; application icon.
SetupIconFile={#BundleDir}\happy-downloader.ico
UninstallDisplayIcon={app}\{#AppExeName}

[Languages]
; The wizard is in English because Inno Setup ships no Chinese translation —
; it is a separate file that has to be vendored, and a missing one stops the
; build. The instructions that matter are written in both languages on the
; ready page, and the application itself is in Chinese.
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
; The application is renamed on the way in: the release's name carries the
; platform because the release holds the binaries for three of them, and
; inside this directory there is only one.
Source: "{#BundleDir}\happy-downloader_windows-amd64.exe"; DestDir: "{app}"; DestName: "{#AppExeName}"; Flags: ignoreversion
Source: "{#BundleDir}\yt-dlp.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BundleDir}\ffmpeg.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BundleDir}\README.txt"; DestDir: "{app}"; DestName: "README.txt"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"; IconFilename: "{app}\{#AppExeName}"; IconIndex: 0
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; IconFilename: "{app}\{#AppExeName}"; IconIndex: 0; Tasks: desktopicon

[Run]
; Started at the end, and it opens the browser itself: the application has no
; console and no window, so the address it listens on is not printed anywhere
; that a user would see.
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; A running copy would hold the port and the files open, and the uninstaller
; would fail to delete them.
Filename: "{cmd}"; Parameters: "/C taskkill /IM {#AppExeName} /F /T >NUL 2>&1"; Flags: runhidden; RunOnceId: "StopApp"

[Code]
const
  { The settings live beside the application's other files, not in the
    directory it was installed into: %APPDATA% belongs to the user and
    survives moving or reinstalling the program. }
  ConfigSubDir = 'happy-downloader';

var
  ConfigWritten: Boolean;

function ConfigPath(): String;
begin
  Result := ExpandConstant('{userappdata}\' + ConfigSubDir + '\config.yml');
end;

{ A YAML value for a Windows path.

  Single quotes are YAML's literal style — backslashes are not escapes in it —
  and a single quote inside is written twice. Nothing else in a path needs
  quoting. }
function YamlPath(const Value: String): String;
var
  Escaped: String;
begin
  Escaped := Value;
  { StringChangeEx changes the string it is given and returns how many
    replacements it made, so the result is read from Escaped. }
  StringChangeEx(Escaped, '''', '''''', True);
  Result := '''' + Escaped + '''';
end;

{ A running copy is stopped before anything is replaced.

  Two things go wrong when it is not. Its files cannot be overwritten, which
  surfaces as a sharing violation part way through the install; and if they
  could, the new process would find the port already taken by the old one, so
  the browser would be opened on the version that was just replaced. }
procedure StopRunningCopy();
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{cmd}'), '/C taskkill /IM {#AppExeName} /F /T >NUL 2>&1',
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

  { The port is released a moment after the process ends. }
  Sleep(700);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopRunningCopy();
  Result := '';
end;

procedure WriteConfig();
var
  Content: String;
  DownloadDir: String;
  AppDir: String;
  ConfigDir: String;
begin
  AppDir := ExpandConstant('{app}');
  { The profile directory is reached through the USERPROFILE environment
    variable. There is no userprofile constant, and asking for one raises
    inside CurStepChanged, where a silent install suppresses the message box
    and carries on as though it had worked. }
  DownloadDir := ExpandConstant('{%USERPROFILE}\Downloads\happy-downloader');
  ConfigDir := ExpandConstant('{userappdata}\' + ConfigSubDir);

  ForceDirectories(ConfigDir);
  ForceDirectories(DownloadDir);

  { An existing file is left alone. It holds the user's settings, and an
    upgrade should not move their downloads back to the default. }
  if FileExists(ConfigPath()) then
  begin
    ConfigWritten := False;
    Exit;
  end;

  Content :=
    '# Written by the installer on ' + GetDateTimeString('yyyy-mm-dd', '-', ':') + '.' + #13#10 +
    '#' + #13#10 +
    '# The settings page can change the download folder, the proxy and the PO' + #13#10 +
    '# Token provider; it writes them back here. The rest is meant to be edited' + #13#10 +
    '# by hand.' + #13#10 +
    'server:' + #13#10 +
    '  host: 0.0.0.0' + #13#10 +
    '  port: 3033' + #13#10 +
    '' + #13#10 +
    'paths:' + #13#10 +
    '  download_path: ' + YamlPath(DownloadDir) + #13#10 +
    '  downloader_path: ' + YamlPath(AppDir + '\yt-dlp.exe') + #13#10 +
    '  ffmpeg_path: ' + YamlPath(AppDir + '\ffmpeg.exe') + #13#10 +
    '  local_database_path: ' + YamlPath(ConfigDir) + #13#10 +
    '' + #13#10 +
    'logging:' + #13#10 +
    '  enable_file_logging: true' + #13#10 +
    '  log_path: ' + YamlPath(ConfigDir + '\happy-downloader.log') + #13#10;

  ConfigWritten := SaveStringToFile(ConfigPath(), Content, False);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    WriteConfig();
end;

{ The ready page says what happens next, because the program it starts has no
  window of its own to say it in. }
function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo,
  MemoTypeInfo, MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
var
  S: String;
begin
  S := '';

  if MemoDirInfo <> '' then
    S := S + MemoDirInfo + NewLine + NewLine;

  S := S + '安装完成后浏览器会自动打开。如果没有打开，请手动访问：' + NewLine + NewLine;
  S := S + Space + 'http://localhost:3033/' + NewLine + NewLine;
  S := S + '程序没有窗口，关闭浏览器不会停止下载。要退出程序，请到设置页最下方的'
    + '「退出程序」。' + NewLine + NewLine;
  S := S + '下载的文件保存在用户目录下的 Downloads\happy-downloader。' + NewLine + NewLine;
  S := S + '可选：在配置文件中设置 cookies.from_browser（例如 chrome、edge、firefox），'
    + '程序将直接读取该浏览器的 cookie，无需导出导入。' + NewLine;
  S := S + 'Happy Downloader runs in the background with no window: closing the'
    + ' browser does not stop it, and the settings page has a quit action.';

  Result := S;
end;
