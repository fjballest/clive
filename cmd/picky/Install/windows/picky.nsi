!include "MUI.nsh"
!include "FileAssociation.nsh"
#define ALL_USERS
!include "WriteEnvStr.nsh"


!define MUI_ICON picky.ico

;Name and file
      Name "Picky installer"
      OutFile "picky.exe"

;--------------------------------
;General
;Name and file

	;Default installation folder
	InstallDir "$APPDATA\picky"
  
	;Get installation folder from registry if available
	InstallDirRegKey HKCU "Software\picky" ""

	;Request application privileges for Windows Vista and successors
;	RequestExecutionLevel user
	RequestExecutionLevel admin
;--------------------------------
;Interface Settings
	

	!define MUI_HEADERIMAGE
;	!define MUI_HEADERIMAGE_BITMAP "..\picky.bmp" ; optional
	!define MUI_ABORTWARNING

;--------------------------------
;pages
	!insertmacro MUI_PAGE_WELCOME
	!insertmacro MUI_PAGE_DIRECTORY
	!insertmacro MUI_PAGE_INSTFILES

	!insertmacro MUI_UNPAGE_CONFIRM
	!insertmacro MUI_UNPAGE_INSTFILES



;---------------------------------
;Languages
 
	!insertmacro MUI_LANGUAGE "English"

;--------------------------------
;Installer Sections

!define REG_UNINSTALL "Software\Microsoft\Windows\CurrentVersion\Uninstall\Picky"


Section "Dummy Section" SecDummy
	SetOutPath "$INSTDIR"
  
	file picky.ico
	file pam.exe
	file pick.exe
	file SciTE.exe
	file SciTEGlobal.properties
	file LicenseSciTE.txt
	CreateShortCut "$DESKTOP\SciTE.lnk" "$INSTDIR\SciTE.exe"
	CreateShortCut "$DESKTOP\picky.lnk" "$INSTDIR" "" "$INSTDIR/picky.ico"
  
	;Store installation folder
	WriteRegStr HKCU "Software\Picky" "" $INSTDIR
	WriteRegStr HKLM "${REG_UNINSTALL}" "DisplayName" "Remove Picky"
	WriteRegStr HKLM "${REG_UNINSTALL}" "UninstallString" "$INSTDIR\Uninstall.exe"
	Push "PICKYDIR"
	Push $INSTDIR
	Call WriteEnvStr
  
	;file type icon
	;http://stackoverflow.com/questions/708238/how-do-i-add-an-icon-to-a-mingw-gcc-compiled-executable
	${registerExtension} "$INSTDIR\pam.exe" ".pam" "Picky_Program"

	;Create uninstaller
	WriteUninstaller "$INSTDIR\Uninstall.exe"

SectionEnd

;--------------------------------
;Descriptions
;Language strings
	LangString DESC_SecDummy ${LANG_ENGLISH} "A test section."

	;Assign language strings to sections
	!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
	!insertmacro MUI_DESCRIPTION_TEXT ${SecDummy} $(DESC_SecDummy)
	!insertmacro MUI_FUNCTION_DESCRIPTION_END



;--------------------------------
;Uninstaller Section
Section "Uninstall"

	Push "PICKYDIR"
	Call un.DeleteEnvStr
	Delete "$DESKTOP\picky.lnk"
	Delete "$DESKTOP\SciTE.lnk"

	DeleteRegKey /ifempty HKCU "Software\picky"
	DeleteRegKey /ifempty HKLM "${REG_UNINSTALL}"

	${unregisterExtension} ".pam" "Picky_Program"

	RMDIR /r "$INSTDIR"

SectionEnd
