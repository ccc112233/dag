import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { getToken } from "./api/client";
import { LoginPage } from "./pages/LoginPage";
import { OverviewPage } from "./pages/OverviewPage";
import { ProtocolsPage } from "./pages/ProtocolsPage";
import { SouthboundListPage } from "./pages/SouthboundListPage";
import { SouthboundFormPage } from "./pages/SouthboundFormPage";
import { SouthboundDetailPage } from "./pages/SouthboundDetailPage";
import { ThingModelsPage } from "./pages/ThingModelsPage";
import { ThingModelEditPage } from "./pages/ThingModelEditPage";
import { NorthboundListPage } from "./pages/NorthboundListPage";
import { NorthboundFormPage } from "./pages/NorthboundFormPage";
import { NorthboundDetailPage } from "./pages/NorthboundDetailPage";
import { SystemPage } from "./pages/SystemPage";

function RequireAuth({ children }: { children: React.ReactNode }) {
  if (!getToken()) return <Navigate to="/login" replace />;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        }
      >
        <Route index element={<OverviewPage />} />
        <Route path="southbound" element={<SouthboundListPage />} />
        <Route path="southbound/new" element={<SouthboundFormPage />} />
        <Route path="southbound/:id" element={<SouthboundDetailPage />} />
        <Route path="thing-models" element={<ThingModelsPage />} />
        <Route path="thing-models/new" element={<ThingModelEditPage />} />
        <Route path="thing-models/:id" element={<ThingModelEditPage />} />
        <Route path="northbound" element={<NorthboundListPage />} />
        <Route path="northbound/new" element={<NorthboundFormPage />} />
        <Route path="northbound/:id" element={<NorthboundDetailPage />} />
        <Route path="protocols" element={<ProtocolsPage />} />
        <Route path="system" element={<SystemPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
