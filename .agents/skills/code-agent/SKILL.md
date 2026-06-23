# Code Agent Skill

## Language Selection
- English
- Español

## Purpose
This skill implements changes in an existing codebase following a structured approach that includes repository reconnaissance, planning, implementation, and verification. It ensures changes are correct, secure, simple, and minimal.

## Activation Criteria
Use this skill when you want to:
- Implement a new feature in an existing codebase
- Make modifications to existing code following best practices
- Add functionality that requires multiple file changes
- Ensure code changes match existing conventions and patterns
- Perform verification to confirm implementation correctness

## Instructions
1. Select your preferred language (English or Español)
2. Provide the implementation request or description of what needs to be implemented
3. Optionally specify any particular constraints or requirements
4. The skill will guide you through the implementation process using the Code Agent framework

## Expected Behavior
When activated, this skill will:
- Guide you through Phase 1: Repository Reconnaissance to understand the codebase
- Help you create an Implementation Plan in Phase 2
- Execute the implementation in Phase 3 following strict constraints
- Conduct verification in Phase 4 to confirm correctness
- Ensure changes are minimal, secure, and follow existing conventions
- Provide output in the selected language

## Output Guidelines
- Output will be in the selected language (English or Español)
- Follows the 4-phase implementation process:
  1. Repository Reconnaissance (reading directory trees, file headers)
  2. Implementation Plan (stating what files will be created/modified)
  3. Implementation (applying constraints: minimalism, code quality, dependency hygiene, tests)
  4. Verification Checklist (confirming implementation meets requirements)
- Includes specific file changes with diffs or full file content
- Groups output in dependency order (dependencies before dependents)
- Avoids summaries/explanations unless a decision requires justification

## Examples

### English Example
**Input:**
```
Goal: Add a logout button to the user profile page
Audience: Frontend developers
Constraints: Should match existing button styles, include confirmation dialog, and call the logout API endpoint
```

**Output:**
The skill will guide you through the four phases:
1. Repository Reconnaissance: Identify entry points, project structure, language/runtime, dependencies, conventions, tests, and configuration
2. Implementation Plan: Specify which files will be created/modified (e.g., user profile component, API service, styles)
3. Implementation: Execute the plan with minimal changes, matching existing patterns, handling errors explicitly
4. Verification Checklist: Confirm the implementation does exactly what was requested, no existing behavior altered, no unnecessary dependencies added, error paths handled, and if tests exist, they were added/updated

### Español Example
**Entrada:**
```
Goal: Agregar un botón de cierre de sesión a la página de perfil de usuario
Audiencia: Desarrolladores frontend
Restricciones: Debe coincidir con los estilos existentes de botones, incluir diálogo de confirmación, y llamar al endpoint de API de cierre de sesión
```

**Salida:**
La habilidad te guiará a través de las cuatro fases:
1. Reconocimiento del Repositorio: Identificar puntos de entrada, estructura del proyecto, lenguaje/tiempo de ejecución, dependencias, convenciones, pruebas y configuración
2. Plan de Implementación: Especificar qué archivos se crearán/modificarán (por ejemplo, componente de perfil de usuario, servicio de API, estilos)
3. Implementación: Ejecutar el plan con cambios mínimos, coincidiendo con patrones existentes, manejando errores explícitamente
4. Lista de Verificación: Confirmar que la implementación haga exactamente lo solicitado, que no se altere el comportamiento existente, que no se agreguen dependencias innecesarias, que se manejen las rutas de error, y que si existen pruebas, se hayan agregado/actualizado