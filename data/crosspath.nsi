!define VERSION 0.2.0
!define NAME crosspath
!define ENTRY Software\${NAME}
!define REPO "https://github.com/busoc/inspect"
!define DEV BUSOCGC

Name "${NAME} installer v${VERSION}"
OutFile "${NAME}-setup.exe"
InstallDir $PROGRAMFILES\${NAME}

ShowInstDetails show
ShowUnInstDetails show

Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "Installer"

SetOutPath $INSTDIR
WriteUninstaller $INSTDIR\uninstall.exe

File ..\..\..\..\..\bin\crosspath.exe

WriteRegStr HKCU ${ENTRY} "DisplayName" ${NAME}
DetailPrint "adding to registryDisplayName(${NAME}) in ${ENTRY}"
WriteRegStr HKCU ${ENTRY} "DisplayVersion" ${VERSION}
DetailPrint "adding to registryDisplayVersion(${VERSION}) in ${ENTRY}"
WriteRegStr HKCU ${ENTRY} "Maintainer" ${DEV}
DetailPrint "adding to registryMaintainer(${DEV}) in ${ENTRY}"
WriteRegStr HKCU ${ENTRY} "Repository" ${REPO}
DetailPrint "adding to registryRepository(${REPO}) in ${ENTRY}"
WriteRegStr HKCU ${ENTRY} "UninstallString" $INSTDIR\uninstall.exe
DetailPrint "adding to registryUnsinstallString() in ${ENTRY}"

createdirectory $INSTDIR\samples
File /oname=$INSTDIR\samples\multi-area.toml ..\..\..\..\..\tmp\cpconfig1.toml
File /oname=$INSTDIR\samples\single-area.toml ..\..\..\..\..\tmp\cpconfig2.toml

SectionEnd

Section "Uninstall"

Delete $INSTDIR\uninstall.exe
Delete $INSTDIR\crosspath.exe
RMDir /r $INSTDIR\samples

DeleteRegKey HKCU ${ENTRY}
DetailPrint "deleting from registry ${ENTRY}"

SectionEnd
