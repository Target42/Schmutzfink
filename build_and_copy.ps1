cd D:\Sourcen\Schmutzfink

cd mobile
flutter build apk --release
cd ..

# falls noch nicht aktuell:
.\mobile\scripts\publish-apk.ps1

.\scripts\build-linux.ps1

scp dist\cache\bin\schmutzfink root@nextcloud:/tmp/schmutzfink
scp data\mobile\schmutzfink-android.apk root@nextcloud:/tmp/
scp data\mobile\schmutzfink-android.json root@nextcloud:/tmp/

# cp /tmp/schmutzfink-android.apk /opt/schmutzfink/data/mobile/
# cp /tmp/schmutzfink-android.json /opt/schmutzfink/data/mobile/
# chown -R schmutzfink:schmutzfink /opt/schmutzfink/data/mobile