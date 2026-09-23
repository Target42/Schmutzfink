import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth.tsx'
import { DetailPage } from './pages/Detail.tsx'
import { Layout } from './pages/Layout.tsx'
import { ListPage } from './pages/List.tsx'
import { LoginPage } from './pages/Login.tsx'
import { MapPage } from './pages/MapPage.tsx'
import { RequireAdmin, RequireAuth, RequireDeletionOfficer, RequireWriter } from './pages/RequireAuth.tsx'
import { UploadPage } from './pages/Upload.tsx'
import { AuditPage } from './pages/Audit.tsx'
import { CasePage } from './pages/Case.tsx'
import { CasesPage } from './pages/Cases.tsx'
import { MotifPage } from './pages/Motif.tsx'
import { MotifsPage } from './pages/Motifs.tsx'
import { UsersPage } from './pages/Users.tsx'
import { DeletionsPage } from './pages/Deletions.tsx'
import { FieldsPage } from './pages/Fields.tsx'
import { TokensPage } from './pages/Tokens.tsx'
import { HelpPage } from './pages/Help.tsx'
import { MobileAppPage } from './pages/MobileApp.tsx'
import { StatsPage } from './pages/Stats.tsx'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<RequireAuth />}>
            <Route element={<Layout />}>
              <Route path="/" element={<ListPage />} />
              <Route path="/karte" element={<MapPage />} />
              <Route path="/auswertungen" element={<StatsPage />} />
              <Route path="/token" element={<TokensPage />} />
              <Route path="/hilfe" element={<HelpPage />} />
              <Route path="/motive" element={<MotifsPage />} />
              <Route path="/motiv/:id" element={<MotifPage />} />
              <Route path="/vorgaenge" element={<CasesPage />} />
              <Route path="/vorgang/:id" element={<CasePage />} />
              <Route path="/datensatz/:id" element={<DetailPage />} />
              <Route element={<RequireWriter />}>
                <Route path="/hochladen" element={<UploadPage />} />
                <Route path="/app" element={<MobileAppPage />} />
                <Route path="/datensatz/:id/bearbeiten" element={<DetailPage />} />
              </Route>
              <Route element={<RequireDeletionOfficer />}>
                <Route path="/loeschungen" element={<DeletionsPage />} />
              </Route>
              <Route element={<RequireAdmin />}>
                <Route path="/benutzer" element={<UsersPage />} />
                <Route path="/felder" element={<FieldsPage />} />
                <Route path="/protokoll" element={<AuditPage />} />
              </Route>
            </Route>
          </Route>
          <Route path="/passwort" element={<Navigate to="/" replace />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
