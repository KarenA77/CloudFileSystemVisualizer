import React, { useState, useEffect } from 'react';
import './Discos.css'; 

function Discos() {
  const [discos, setDiscos] = useState([]);
  const [showPartitions, setShowPartitions] = useState(true); // Estado para mostrar/esconder particiones
  const [error, setError] = useState(''); // Para manejar errores

  // Función para obtener los discos desde el backend
  const obtenerDiscos = async () => {
    try {
      const response = await fetch('http://localhost:8080/discos', {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      if (!response.ok) {
        throw new Error(`Network response was not ok: ${response.statusText}`);
      }

      const data = await response.json();
      if (data.length === 0) {
        throw new Error('No hay discos disponibles en la respuesta.');
      }

      setDiscos(data);
      //console.log('Discos obtenidos:', data);
    } catch (error) {
      console.error('Error al obtener los discos:', error);
      setError(`Error al obtener los discos: ${error.message}`);
    }
  };

  useEffect(() => {
    obtenerDiscos();
  }, []);

  // Función para alternar la visibilidad de las particiones
  const togglePartitions = () => {
    setShowPartitions(!showPartitions);
  };

  return (
    <div className="container">
      <div className="section-header">Sistema de Discos</div>
      <div className="disk-row">
        {discos.map((disco, index) => (
          <div key={index} className="disco">
            <img src="/images/hard_drive.png" alt="Disco" className="disco-img" />
            <div className="disco-info">
              <p className="disco-name">{`Disco ${index + 1}`}</p>
              <p className="disco-details">{`Tamaño: ${disco.size} ${disco.unit.toUpperCase()}`}</p>
              <p className="disco-details">{`Fit: ${disco.fit.toUpperCase()}`}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Sección de Particiones con botón para esconder */}
      <div className="partition-header">
        <span>Sistema de Particiones</span>
        {/* Botón con flecha */}
        <button className="toggle-arrow-btn" onClick={togglePartitions}>
          {showPartitions ? '▲' : '▼'}
        </button>
      </div>

      {/* Sección de Particiones condicional */}
      {showPartitions && (
        <div className="partition-row">
          {discos.map((disco, index) => (
            <div key={index} className="partitions-container">
              <h3>{`Particiones del Disco ${index + 1}`}</h3>
              {disco.particiones?.length > 0 ? (
                  disco.particiones.map((particion, pIndex) => (
                    <div key={pIndex} className="particion">
                      <p><strong>{`Partición ${pIndex + 1}`}</strong></p>
                      <p>{`Nombre: ${particion.name || 'Desconocida'}`}</p>
                      <p>{`Tamaño: ${particion.size || 'N/A'} ${particion.unit?.toUpperCase() || ''}`}</p>
                      <p>{`Tipo: ${particion.type?.toUpperCase() || ''}`}</p>
                    </div>
                  ))
                ) : (
                  <p>No hay particiones creadas en este disco</p>
                )}

            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default Discos;
