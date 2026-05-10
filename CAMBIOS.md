

## 1. ¿Qué tipo de dashboard? (No es app web)

Para tu caso (reporte interactivo, no live, muchos datos), lo mejor es un **dashboard HTML estático pero interactivo** generado periódicamente (ej. cada día o bajo demanda). Ventajas:
- Lo abres en tu navegador desde tu disco local.
- Puedes usar **Plotly** (gráficos interactivos: zoom, hover, selección) + **D3.js**.
- Todo el procesamiento pesado lo haces offline con Python (pandas, numpy) y solo guardas resultados agregados + JSON para los gráficos.
- Súper rápido, no requiere servidor, fácil de compartir.

Alternativa: **Jupyter Notebook con Voilà** (convierte notebooks en dashboards estáticos interactivos). Pero yo me quedaría con un script Python que genera un `reporte.html` con Plotly.

## 2. Información a mostrar (yendo más allá de lo básico)

Tus logs tienen profundidad: batería, CPU, temp, procesos, conectividad, etc. Aquí ideas que **correlacionan** y **descubren anomalías**:

### 2.1. Salud y degradación de batería (no solo %)
- **Evolución del wear_level** vs ciclos y tiempo. Proyección a futuro (cuándo llegará a 20%, 30%).
- **Capacidad efectiva por ciclo**: ajusta la capacidad actual vs ciclos. ¿Hay saltos bruscos?
- **Tasa de auto-descarga** (cuando está suspendido o apagado? – aunque tu log solo cuando está prendido, podrías inferir de la primera muestra del día).
- **Voltaje vs porcentaje**: desviación de la curva teórica. Si el voltaje cae antes de tiempo, batería envejecida.

### 2.2. Eficiencia energética y patrones de uso
- **Energy-rate normalizado por carga de CPU**: (`energy-rate / load_avg`). Así detectas si el sistema consume más energía de lo esperado para la misma carga (drivers, procesos colgados, calor).
- **Tiempo restante real vs predicho**: comparas `time to empty` con lo que realmente duró hasta que conectó cargador. Calculas error absoluto medio. ¿Subestima sobrestima?
- **Perfiles de potencia**: cuánto tiempo usas `power-saver`, `balanced`, `performance`. Y la eficiencia en cada uno.

### 2.3. Correlaciones cruzadas (joyas ocultas)
- **Temperatura CPU vs tasa de descarga**: deberían subir juntas. Si no, algo raro (sensor, throttling mal configurado).
- **Procesos que más impacto tienen en batería**: no por %CPU bruto, sino por **aumento de energy-rate** cuando aparecen. Detecta procesos vampiro.
- **Brightness vs energy-rate**: ajustando por load. ¿Tu pantalla consume mucho más de lo que debería?
- **WiFi/Bluetooth on vs descarga**: diferencia estadística. Puede que el Bluetooth tenga un costo oculto.

### 2.4. Anomalías y mini-advertencias (sin ser alertas en vivo)
- **Sesiones con descarga anormalmente rápida** (percentil 95 de rate para un load dado). Marcarlas en el timeline.
- **Picos de temperatura seguidos de throttling** (aunque tú no lo veas, podrías inferir si frecuencia baja mientras temp alta).
- **Cambios bruscos de voltage** (puede indicar conexión sucia o batería fallando).
- **Procesos que consumen mucha memoria swap**: degrada rendimiento y batería.

### 2.5. Análisis temporal (días, semanas, meses)
- **Calendario de calor** de porcentaje de batería usado por día (cuánto descargaste en total).
- **Ciclos de carga/descarga**: duración media, profundidad de descarga (¿llegas a 0% seguido? malo para salud).
- **Estacionalidad semanal**: los fines de semana consumes más batería? ¿usas perfiles distintos?
- **Tendencias a largo plazo**: wear, tiempo medio por sesión, load promedio.

## 3. Cómo organizar la visualización (evitando saturación)

Tienes razón, hay mucha info. La clave es **jerarquía y pestañas**.

Propuesta de estructura del dashboard HTML:

### Pestaña 1: **Resumen ejecutivo** (una mirada rápida)
- Métricas clave actuales (último registro): wear, ciclos, eficiencia promedio.
- Gráfico de línea de **wear_level** desde oct 2025 hasta hoy (con proyección lineal).
- **Semáforo de salud**: verde (wear<10%), amarillo (10-20%), rojo (>20%). Con fecha estimada de llegar a 20% y 30%.
- Top 3 **procesos más dañinos históricos** (mayor aumento de energy-rate).

### Pestaña 2: **Evolución y tendencias**
- **Capacidad restante (Wh) vs ciclos** (dispersión). Línea de regresión.
- **Tasa de descarga promedio por mes** (ajustada por load). ¿Ha empeorado la eficiencia?
- **Temperatura media por hora del día**, superpuesta con descarga media.

### Pestaña 3: **Correlaciones interactivas**
- **Matriz de correlación** (energy-rate, load, temp, brightness, voltage). Calor interactivo.
- **Gráfico de dispersión** seleccionable: eje X = load_avg, eje Y = energy-rate, color = temp. Filtro por rango de fechas.
- **Línea de tiempo sincronizada**: abajo un slider de fechas que actualiza todas las gráficas.

### Pestaña 4: **Procesos y vampiro energético**
- **Tabla de procesos** con: %CPU promedio, aumento de energy-rate (cuando están activos vs ausentes), memoria, frecuencia de aparición.
- **Gráfico de barras** del impacto energético neto (W extra por hora que ese proceso está corriendo).

### Pestaña 5: **Análisis de sesiones**
- **Cada vez que desconectas cargador** se inicia una sesión. Aquí puedes mostrar:
  - Duración, porcentaje consumido, tasa promedio, temp máxima, proceso principal.
  - **Detección de sesiones anómalas** (las resalta en rojo): duración menor de 30 min con alta descarga, o calentamiento excesivo.
- Histograma de duraciones de sesión.

### Pestaña 6: **Línea de tiempo completa (zoom-in)**
- Un gráfico de velas (o área) con **tasa de descarga** y **temperatura** superpuestos. Rango seleccionable para hacer zoom en días/horas.
- Marcadores automáticos: "aquí hubo un pico de descarga", "aquí la batería perdió 2% de capacidad".

## 4. Implementación práctica (arquitectura)

Dado que tienes logs por sesión, por mes y total, lo más eficiente es:

```python
# 1. Preprocesado (se ejecuta una vez al día o al generar reporte)
import pandas as pd
import numpy as np

# Cargar el archivo total (o concatenar los mensuales)
df = pd.read_csv('logs_totales.csv', parse_dates=['timestamp'])

# 2. Agregar features derivados
df['load_squared'] = df['load_avg']**2  # para detectar no linealidad
df['energy_per_load'] = df['energy-rate'] / (df['load_avg'] + 0.1)

# Detectar sesiones de descarga (asumiendo que cambia a 'charging' en algún momento)
# ... lógica de cambio de estado

# 3. Calcular agregados diarios / horarios (para reducir tamaño y velocidad)
daily = df.resample('D', on='timestamp').agg({
    'energy-rate': 'mean',
    'load_avg': 'mean',
    'cpu_temp': 'max',
    'wear_level': 'last',
    'cycles': 'last'
})

# 4. Guardar JSONs para Plotly
daily.to_json('data/daily.json', orient='records')
df_sample = df.sample(frac=0.1)  # para scatter plots grandes
df_sample.to_json('data/sample.json', orient='records')
```

Luego usas **Plotly.js** en HTML (o Python + `plotly.offline.plot`). Puedes generar todo con un script que produce un solo `dashboard.html`.

## 5. Ejemplo concreto de "mini advertencia" (no alerta, sino insight)

En el resumen ejecutivo podrías poner una tarjeta como:

> **Advertencia de eficiencia:** Los últimos 7 días, tu consumo por unidad de carga CPU ha subido un 12% respecto a tu media histórica. Posible causa: actualizaciones recientes o calor ambiental. Revisa procesos como `gnome-software` que aparecen más seguido.

O:

> **Correlación interesante:** Cuando el bluetooth está encendido, consumes en promedio 1.2 W más para el mismo nivel de carga. Si no lo usas, apágalo.

## 6. ¿Y si no quieres programar tanto?

Puedes usar **Redash** o **Metabase** conectado a SQLite (conviertes logs a SQLite). Pero perderías interactividad fina de Plotly. Yo recomendaría el script HTML, porque lo personalizas 100% y no depende de servicios.

## Conclusión: qué te recomiendo hacer AHORA

1. **Elige el enfoque**: script Python que genera `dashboard.html` con Plotly subplots y pestañas (uso `html` + `plotly.graph_objects`).
2. **Procesa los datos** para obtener las agregaciones diarias/horarias (no cargues 6 meses de logs de 60s en el navegador, serían ~260k puntos, manejable pero pesado. Mejor agregar).
3. **Implementa 2-3 gráficos clave primero**: wear vs ciclos, tasa descarga vs load, y evolución temporal.
4. **Añade las pestañas progresivamente** a medida que exploras y encuentras correlaciones interesantes.