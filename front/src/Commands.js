import React, { useState } from 'react';
import './Commands.css';


function App() {
  const [inputText, setInputText] = useState('');
  const [outputText, setOutputText] = useState('');
  const [fileName, setFileName] = useState('');

  // Función para manejar la ejecución de los comandos
  const handleExecute = async () => {
    console.log("Ejecutando solicitud POST a /execute con inputText:", inputText);

    // Dividir inputText en un array de comandos separados por saltos de línea o comas
    const commands = inputText.split(/\r?\n|\r|,/).map(cmd => cmd.trim()).filter(cmd => cmd !== '');

    try {
      const response = await fetch('http://localhost:8080/execute', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(commands), // Enviar los comandos como un array
      });

      if (!response.ok) {
        throw new Error('Network response was not ok');
      }

      const data = await response.json();
      
      // Mostrar toda la respuesta recibida en la salida
      setOutputText(data.messages.join('\n')); // Unir los mensajes en un solo string
      console.log("Respuesta recibida del servidor:", data);
    } catch (error) {
      console.error('Error:', error);
      setOutputText("Error al ejecutar los comandos.");
    }
  };

  // Función para manejar la carga de archivos
  const handleFileChange = (event) => {
    if (event.target.files.length > 0) {
      const file = event.target.files[0];
      setFileName(file.name);

      const reader = new FileReader();
      reader.onload = (e) => {
        setInputText(e.target.result);
      };
      reader.readAsText(file);
    }
  };

  return (
    <div className="App">
      <header className="App-header">
        <h1>Proyecto 1 MIA</h1>
        <div className="input-output-container">
          <div className="input-box">
            <h2 className="box-title">Entrada</h2>
            <textarea
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              rows="10"
            />
          </div>
          <div className="output-box">
            <h2 className="box-title">Salida</h2>
            <textarea
              value={outputText}
              readOnly
              rows="10"
            />
          </div>
          <div className="buttons">
            <button
              className="execute-button"
              onClick={handleExecute}
            >
              Ejecutar
            </button>
            <input
              type="file"
              id="file-input"
              onChange={handleFileChange}
              style={{ display: 'none' }}
            />
            <label htmlFor="file-input" className="file-button">
              Cargar Archivo
            </label>
            {fileName && <p className="file-name">Archivo: {fileName}</p>}
          </div>
        </div>
      </header>
    </div>
  );
}

export default App;


