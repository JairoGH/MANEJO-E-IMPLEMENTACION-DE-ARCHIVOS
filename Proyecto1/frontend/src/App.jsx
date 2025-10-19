import React, { useEffect, useRef, useState } from "react";

// Componente de Header con navegación
const Header = ({ activeTab, setActiveTab }) => {
  const tabs = [
    { id: 'console', label: 'Consola', icon: '💻' },
    { id: 'reports', label: 'Reportes', icon: '📊' },
    { id: 'about', label: 'Acerca de', icon: 'ℹ️' }
  ];

  return (
    <header className="bg-black border-b border-green-500">
      <div className="max-w-6xl mx-auto px-6 py-4">
        <div className="flex flex-col space-y-4">
          <div>
            <h1 className="text-4xl font-bold text-green-400">GoDisk</h1>
            <h2 className="text-2xl font-bold text-green-600">Proyecto MIA 201902672</h2>
          </div>
          
          <nav className="flex space-x-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center space-x-2 px-4 py-2 rounded-lg transition ${
                  activeTab === tab.id
                    ? 'bg-green-500 text-black font-semibold'
                    : 'text-green-400 hover:bg-black-900 hover:text-green-300'
                }`}
              >
                <span className="text-lg">{tab.icon}</span>
                <span>{tab.label}</span>
              </button>
            ))}
          </nav>
        </div>
      </div>
    </header>
  );
};

// Componente de Menú de Archivo
const FileMenu = ({ onOpenFile, onSaveFile, onSaveAsFile, currentFileName }) => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center space-x-2 px-4 py-2 bg-black-900 text-green-400 border border-green-500 rounded hover:bg-green-500 hover:text-black transition"
      >
        <span>📄</span>
        <span>Archivo</span>
      </button>
      
      {isOpen && (
        <>
          <div 
            className="fixed inset-0 z-10" 
            onClick={() => setIsOpen(false)}
          />
          <div className="absolute top-full left-0 mt-1 bg-black border border-green-500 rounded-lg shadow-lg z-20 min-w-48">
            <button
              onClick={() => {
                onOpenFile();
                setIsOpen(false);
              }}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-black-900 rounded-t-lg"
            >
              <span>📂</span>
              <span>Abrir archivo...</span>
            </button>
            
            <button
              onClick={() => {
                onSaveFile();
                setIsOpen(false);
              }}
              disabled={!currentFileName}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-black-900 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span>💾</span>
              <span>Guardar</span>
              {currentFileName && <span className="text-xs text-green-600">({currentFileName})</span>}
            </button>
            
            <button
              onClick={() => {
                onSaveAsFile();
                setIsOpen(false);
              }}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-black-900 rounded-b-lg"
            >
              <span>⬇️</span>
              <span>Guardar como...</span>
            </button>
          </div>
        </>
      )}
    </div>
  );
};

// Componente de la Consola
const ConsoleTab = () => {
  const [entradaComandos, setEntradaComandos] = useState("");
  const [salidaComandos, setSalidaComandos] = useState("");
  const [loading, setLoading] = useState(false);
  const [currentFileName, setCurrentFileName] = useState("");
  const outRef = useRef(null);
  const fileInputRef = useRef(null);

  // Autoscroll en la salida
  useEffect(() => {
    outRef.current?.scrollTo({ top: outRef.current.scrollHeight });
  }, [salidaComandos, loading]);

  // Atajo Ctrl/Cmd + Enter
  useEffect(() => {
    const h = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
        e.preventDefault();
        manejarEjecucion();
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  });

  const manejarEjecucion = async () => {
    const command = entradaComandos.trim();
    if (!command || loading) return;

    try {
      setLoading(true);
      setSalidaComandos((prev) => prev + `\n▶ Ejecutando...\n`);

      const respuesta = await fetch("http://localhost:3001/execute", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command }),
      });

      if (!respuesta.ok) {
        const t = await respuesta.text();
        throw new Error(t || `HTTP ${respuesta.status}`);
      }

      const datos = await respuesta.json();
      const out = (datos?.output ?? "").toString().replace(/\r/g, "");
      setSalidaComandos((prev) => prev + out + `\n✔ Finalizado\n`);
    } catch (e) {
      setSalidaComandos((prev) => prev + `\n✖ Error: ${e.message || e}\n`);
    } finally {
      setLoading(false);
    }
  };

  const handleOpenFile = () => {
    fileInputRef.current?.click();
  };

  const manejarArchivo = async (evento) => {
    const archivo = evento.target.files?.[0];
    if (!archivo) return;
    const contenido = await archivo.text();
    setEntradaComandos(contenido);
    setCurrentFileName(archivo.name);
    evento.target.value = "";
  };

  const handleSaveFile = () => {
    if (!currentFileName) {
      handleSaveAsFile();
      return;
    }
    
    const blob = new Blob([entradaComandos], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = currentFileName;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleSaveAsFile = () => {
    const fileName = prompt('Nombre del archivo:', currentFileName || 'script.smia');
    if (!fileName) return;
    
    const blob = new Blob([entradaComandos], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(url);
    setCurrentFileName(fileName);
  };

  const limpiarSalida = () => setSalidaComandos("");
  
  const copiarSalida = async () => {
    try { 
      await navigator.clipboard.writeText(salidaComandos); 
      alert('Salida copiada al portapapeles');
    } catch {
      alert('Error al copiar al portapapeles');
    }
  };

  // Drag & drop
  const onDrop = async (e) => {
    e.preventDefault();
    const file = e.dataTransfer.files?.[0];
    if (!file) return;
    setEntradaComandos(await file.text());
    setCurrentFileName(file.name);
  };
  
  const onDragOver = (e) => e.preventDefault();

  return (
    <div className="space-y-6">
      {/* Barra de herramientas */}
      <div className="flex flex-wrap items-center gap-3 p-4 bg-black-900 border border-green-500 rounded-lg">
        <FileMenu 
          onOpenFile={handleOpenFile}
          onSaveFile={handleSaveFile}
          onSaveAsFile={handleSaveAsFile}
          currentFileName={currentFileName}
        />
        
        <div className="w-px h-8 bg-green-500" />
        
        <button
          onClick={manejarEjecucion}
          disabled={loading || !entradaComandos.trim()}
          className={`flex items-center space-x-2 px-4 py-2 rounded transition ${
            loading
              ? "bg-black-800 text-black-500 cursor-not-allowed"
              : "bg-green-500 text-black font-semibold hover:bg-green-400"
          }`}
        >
          <span>{loading ? "⏳" : "▶️"}</span>
          <span>{loading ? "Ejecutando..." : "Ejecutar"}</span>
        </button>

        <p className="text-xs text-green-600 ml-auto">
          <kbd className="px-2 py-1 bg-black border border-green-500 rounded text-xs text-green-400">Ctrl/Cmd + Enter</kbd> para ejecutar
        </p>
      </div>

      {/* Editor de comandos */}
      <div
        onDrop={onDrop}
        onDragOver={onDragOver}
        className="rounded-lg border border-green-500 bg-black-900 overflow-hidden"
      >
        <div className="bg-black px-4 py-2 border-b border-green-500">
          <span className="text-sm text-green-400">
            {currentFileName || "Sin título"} - Editor de comandos
          </span>
        </div>
        <textarea
          className="w-full h-64 p-4 bg-transparent text-green-400 placeholder-green-700 outline-none resize-none"
          value={entradaComandos}
          onChange={(e) => setEntradaComandos(e.target.value)}
          placeholder={`Escribe comandos aquí o arrastra un archivo .smia

# Ejemplo:
# mkdisk -size=5 -unit=M -path="/home/Disco1.mia"
# fdisk -size=1 -unit=M -path="/home/Disco1.mia" -type=P -name="Particion1"
# mount -path="/home/Disco1.mia" -name="Particion1"`}
        />
      </div>

      {/* Controles de salida */}
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-green-400">Salida de la consola</h3>
        <div className="flex gap-2">
          <button
            onClick={copiarSalida}
            disabled={!salidaComandos}
            className="px-3 py-1 text-sm border border-green-500 text-green-400 rounded hover:bg-green-500 hover:text-black transition disabled:opacity-50"
          >
            📋 Copiar
          </button>
          <button
            onClick={limpiarSalida}
            disabled={!salidaComandos}
            className="px-3 py-1 text-sm border border-red-500 text-red-400 rounded hover:bg-red-500 hover:text-black transition disabled:opacity-50"
          >
            🗑️ Limpiar
          </button>
        </div>
      </div>

      {/* Salida de consola */}
      <div
        ref={outRef}
        className="bg-black border border-green-500 rounded-lg p-4 h-80 overflow-auto text-green-400 font-mono"
      >
        <pre>{salidaComandos || "# Salida de comandos aparecerá aquí..."}</pre>
      </div>

      {/* Input oculto para archivos */}
      <input
        ref={fileInputRef}
        type="file"
        accept=".smia,.txt"
        onChange={manejarArchivo}
        className="hidden"
      />
    </div>
  );
};

// Componente de Reportes
const ReportsTab = () => {
  const [selectedReport, setSelectedReport] = useState(null);

  const reports = [
    {
      id: 'mbr',
      name: 'Reporte MBR',
      description: 'Visualizar Master Boot Record',
      icon: '💾',
      color: 'bg-green-600'
    },
    {
      id: 'disk',
      name: 'Reporte de Disco',
      description: 'Estructura completa del disco',
      icon: '💿',
      color: 'bg-green-500'
    },
    {
      id: 'inode',
      name: 'Reporte de Inodos',
      description: 'Información de inodos del sistema',
      icon: '🗂️',
      color: 'bg-green-700'
    },
    {
      id: 'journaling',
      name: 'Reporte de Journaling',
      description: 'Registro de transacciones',
      icon: '📋',
      color: 'bg-green-600'
    },
    {
      id: 'tree',
      name: 'Árbol de Directorios',
      description: 'Estructura jerárquica de archivos',
      icon: '🌳',
      color: 'bg-green-500'
    },
    {
      id: 'sb',
      name: 'Super Bloque',
      description: 'Información del sistema de archivos',
      icon: '🧱',
      color: 'bg-green-700'
    }
  ];

  const generateReport = (reportType) => {
    alert(`Generando reporte: ${reportType}\n\nEsta funcionalidad se conectará con el backend para generar el reporte solicitado.`);
  };

  return (
    <div className="space-y-6">
      <div className="bg-black-900 border border-green-500 rounded-lg p-6">
        <div className="flex items-center space-x-3 mb-4">
          <span className="text-2xl">📊</span>
          <h3 className="text-xl font-semibold text-green-400">Generador de Reportes</h3>
        </div>
        <p className="text-green-600 mb-6">
          Selecciona el tipo de reporte que deseas generar para visualizar la estructura del sistema de archivos EXT2.
        </p>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {reports.map((report) => (
            <div
              key={report.id}
              className={`bg-black border border-green-600 rounded-lg p-4 hover:border-green-400 transition cursor-pointer ${
                selectedReport === report.id ? 'border-green-400 bg-green-500/10' : ''
              }`}
              onClick={() => setSelectedReport(report.id)}
            >
              <div className="flex items-start space-x-3">
                <div className={`${report.color} p-3 rounded-lg`}>
                  <span className="text-black text-lg">{report.icon}</span>
                </div>
                <div className="flex-1">
                  <h4 className="font-semibold text-green-400 mb-1">{report.name}</h4>
                  <p className="text-sm text-green-600 mb-3">{report.description}</p>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      generateReport(report.name);
                    }}
                    className="px-3 py-1 text-xs bg-green-500 text-black font-semibold rounded hover:bg-green-400 transition"
                  >
                    Generar
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Área de visualización de reportes */}
      <div className="bg-white-900 border border-green-500 rounded-lg p-6">
        <h3 className="text-lg font-semibold text-green-400 mb-4">Vista Previa del Reporte</h3>
        {selectedReport ? (
          <div className="bg-black border border-green-600 rounded p-4 h-64 overflow-auto">
            <div className="text-center mt-20">
              <div className="text-4xl mb-4">
                {reports.find(r => r.id === selectedReport)?.icon}
              </div>
              <p className="text-green-600">
                Reporte "<strong className="text-green-400">{reports.find(r => r.id === selectedReport)?.name}</strong>" seleccionado.
                <br />
                <span className="text-sm">Haz clic en "Generar" para crear el reporte.</span>
              </p>
            </div>
          </div>
        ) : (
          <div className="bg-black border border-green-600 rounded p-4 h-64 flex items-center justify-center">
            <div className="text-center">
              <span className="text-4xl mb-2 block">📈</span>
              <p className="text-green-600">Selecciona un tipo de reporte para comenzar</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

// Componente Acerca de
const AboutTab = () => {
  const features = [
    "Gestión completa de discos virtuales",
    "Creación y manejo de particiones",
    "Sistema de archivos EXT2 simplificado",
    "Operaciones de archivos y directorios",
    "Generación de reportes visuales",
    "Interfaz intuitiva y moderna"
  ];

  const commands = [
    { cmd: 'mkdisk', desc: 'Crear un disco virtual', icon: '💾' },
    { cmd: 'rmdisk', desc: 'Eliminar un disco virtual', icon: '🗑️' },
    { cmd: 'fdisk', desc: 'Gestionar particiones', icon: '🔧' },
    { cmd: 'mount', desc: 'Montar particiones', icon: '📌' },
    { cmd: 'unmount', desc: 'Desmontar particiones', icon: '📌' },
    { cmd: 'mkfs', desc: 'Formatear particiones', icon: '🔄' },
    { cmd: 'login', desc: 'Iniciar sesión de usuario', icon: '🔑' },
    { cmd: 'logout', desc: 'Cerrar sesión de usuario', icon: '🚪' },
    { cmd: 'mkgrp', desc: 'Crear grupos de usuarios', icon: '👥' },
    { cmd: 'rmgrp', desc: 'Eliminar grupos de usuarios', icon: '👥' },
    { cmd: 'mkusr', desc: 'Crear usuarios', icon: '👤' },
    { cmd: 'rmusr', desc: 'Eliminar usuarios', icon: '👤' },
    { cmd: 'mkdir', desc: 'Crear directorios', icon: '📁' },
    { cmd: 'mkfile', desc: 'Crear archivos', icon: '📄' },
    { cmd: 'rep', desc: 'Generar reportes', icon: '📊' }
  ];

  return (
    <div className="space-y-6">
      <div className="bg-black-900 border border-green-500 rounded-lg p-8">
        <div className="text-center mb-8">
          <span className="text-6xl mb-4 block">🖥️</span>
          <h3 className="text-3xl font-bold text-green-400 mb-2">Simulador EXT2</h3>
          <p className="text-xl text-green-600">Proyecto MIA 2S2025</p>
          <div className="flex items-center justify-center space-x-4 mt-4 text-sm text-green-700">
            <span className="flex items-center space-x-1">
              <span>📚</span>
              <span>Versión 1.0</span>
            </span>
          </div>
        </div>
        
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          <div>
            <h4 className="text-lg font-semibold text-green-400 mb-4 flex items-center space-x-2">
              <span>📋</span>
              <span>Descripción del Proyecto</span>
            </h4>
            <p className="text-green-600 text-sm leading-relaxed mb-4">
              Este simulador implementa las funcionalidades básicas del sistema de archivos EXT2, 
              desarrollado como proyecto para el curso de Manejo e Implementación de Archivos.
            </p>
            
            <h5 className="font-semibold text-green-400 mb-3 flex items-center space-x-2">
              <span>✨</span>
              <span>Características Principales:</span>
            </h5>
            <ul className="text-green-600 text-sm space-y-2">
              {features.map((feature, index) => (
                <li key={index} className="flex items-start space-x-2">
                  <span className="text-green-400 mt-1">•</span>
                  <span>{feature}</span>
                </li>
              ))}
            </ul>
          </div>
          
          <div>
            <h4 className="text-lg font-semibold text-green-400 mb-4 flex items-center space-x-2">
              <span>⚡</span>
              <span>Comandos Soportados</span>
            </h4>
            <div className="bg-black border border-green-600 rounded-lg p-4 max-h-64 overflow-y-auto">
              <div className="space-y-2">
                {commands.map((item, index) => (
                  <div key={index} className="flex items-center justify-between py-1">
                    <div className="flex items-center space-x-2">
                      <span className="text-sm">{item.icon}</span>
                      <code className="text-green-400 font-mono text-sm">{item.cmd}</code>
                    </div>
                    <span className="text-green-600 text-xs">{item.desc}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
        
        <div className="mt-8 pt-6 border-t border-green-700">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h5 className="font-semibold text-green-400 mb-3 flex items-center space-x-2">
                <span>🛠️</span>
                <span>Tecnologías Utilizadas</span>
              </h5>
              <div className="flex flex-wrap gap-2">
                {['React', 'Vite', 'Tailwind CSS', 'Node.js', 'JavaScript'].map((tech) => (
                  <span key={tech} className="px-2 py-1 bg-black border border-green-600 text-green-400 rounded text-xs">
                    {tech}
                  </span>
                ))}
              </div>
            </div>
            <div>
              <h5 className="font-semibold text-green-400 mb-3 flex items-center space-x-2">
                <span>🎓</span>
                <span>Información Académica</span>
              </h5>
              <div className="text-sm text-green-600 space-y-1">
                <p><strong className="text-green-400">Universidad:</strong> Universidad de San Carlos de Guatemala</p>
                <p><strong className="text-green-400">Facultad:</strong> Ingeniería</p>
                <p><strong className="text-green-400">Curso:</strong> Manejo e Implementación de Archivos</p>
                <p><strong className="text-green-400">Período:</strong> Segundo Semestre 2025</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Información adicional */}
      <div className="bg-black-900 border border-green-500 rounded-lg p-6">
        <h4 className="text-lg font-semibold text-green-400 mb-4 flex items-center space-x-2">
          <span>📖</span>
          <span>Instrucciones de Uso</span>
        </h4>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
          <div className="bg-black p-4 rounded border border-green-600">
            <div className="flex items-center space-x-2 mb-2">
              <span className="text-lg">📂</span>
              <h5 className="font-semibold text-green-400">1. Cargar Script</h5>
            </div>
            <p className="text-green-600">Usa el menú "Archivo" para cargar un script .smia o escribe comandos directamente en el editor.</p>
          </div>
          <div className="bg-black p-4 rounded border border-green-600">
            <div className="flex items-center space-x-2 mb-2">
              <span className="text-lg">⚡</span>
              <h5 className="font-semibold text-green-400">2. Ejecutar Comandos</h5>
            </div>
            <p className="text-green-600">Presiona el botón "Ejecutar" o usa Ctrl/Cmd + Enter para procesar los comandos.</p>
          </div>
          <div className="bg-black p-4 rounded border border-green-600">
            <div className="flex items-center space-x-2 mb-2">
              <span className="text-lg">📊</span>
              <h5 className="font-semibold text-green-400">3. Ver Reportes</h5>
            </div>
            <p className="text-green-600">Navega a la sección de "Reportes" para generar visualizaciones del sistema de archivos.</p>
          </div>
        </div>
      </div>
    </div>
  );
};

// Componente principal
export default function App() {
  const [activeTab, setActiveTab] = useState('console');

  const renderActiveTab = () => {
    switch (activeTab) {
      case 'console':
        return <ConsoleTab />;
      case 'reports':
        return <ReportsTab />;
      case 'about':
        return <AboutTab />;
      default:
        return <ConsoleTab />;
    }
  };

  return (
    <div className="min-h-screen bg-black text-green-400">
      <Header activeTab={activeTab} setActiveTab={setActiveTab} />
      <main className="max-w-6xl mx-auto px-6 py-8">
        {renderActiveTab()}
      </main>
    </div>
  );
}