import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/common/Layout';
import Dashboard from './pages/Dashboard';
import Accounts from './pages/Accounts';
import Search from './pages/Search';
import Replies from './pages/Replies';
import Metrics from './pages/Metrics';
import ActivityLogs from './pages/ActivityLogs';
import Settings from './pages/Settings';
import { Toaster } from './components/ui/sonner';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="accounts" element={<Accounts />} />
          <Route path="search" element={<Search />} />
          <Route path="replies" element={<Replies />} />
          <Route path="metrics" element={<Metrics />} />
          <Route path="logs" element={<ActivityLogs />} />
          <Route path="settings" element={<Settings />} />
        </Route>
      </Routes>
      <Toaster />
    </BrowserRouter>
  );
}

export default App;
