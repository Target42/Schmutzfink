cd D:\Sourcen\Schmutzfink

# falls noch nicht aktuell:
.\mobile\scripts\publish-apk.ps1

.\scripts\build-linux.ps1

scp dist\cache\bin\schmutzfink root@nextcloud:/tmp/schmutzfink
scp data\mobile\schmutzfink-android.apk root@nextcloud:/tmp/
scp data\mobile\schmutzfink-android.json root@nextcloud:/tmp/