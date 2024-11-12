import React from 'react';
import { Link } from 'react-router-dom';

const Login = () => {
  return (
    <div>
      <h1>Inicio de Sesión</h1>
      <Link to="/discos">Ir a Discos</Link> {/* Hipervínculo hacia la página de discos */}
    </div>
  );
};

export default Login;
