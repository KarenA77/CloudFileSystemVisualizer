import React from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import Login from './pages/Login';
import Logout from './pages/Logout';
import Commands from './Commands';

import Discos from './pages/Discos';

function App() {
  return (
    <Router>
      <div>
        <Routes>
          <Route path="/" element={<Commands />} />  {/* Ruta raíz para discos del SO */}
          <Route path="/discos" element={<Discos />} /> {/* Ruta para login */}
          <Route path="/logout" element={<Logout />} /> {/* Ruta para logout */}
          <Route path="/login" element={<Login />} /> {/* Ruta para login */}
          
        </Routes>
      </div>
    </Router>
  );
}

export default App;
