package com.bigBrother.api.controllers;

import com.bigBrother.api.dtos.CameraDTO;
import com.bigBrother.api.exceptions.ErrorResponse;
import com.bigBrother.api.exceptions.ResourceNotFoundException;
import com.bigBrother.api.models.CameraModel;
import com.bigBrother.api.repositories.CameraRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.stream.Collectors;

/**
 * Controller for managing camera resources.
 */
@RestController
@RequestMapping("/api/cameras")
public class CameraController {

    @Autowired
    private CameraRepository cameraRepository;

    /**
     * Retrieves all cameras.
     *
     * @return List of CameraDTO objects.
     */
    @GetMapping
    public List<CameraDTO> getAllCameras() {
        return cameraRepository.findAll().stream().map(this::convertToDTO).collect(Collectors.toList());
    }

    /**
     * Creates a new camera if the name is unique.
     *
     * @param camera The camera model to create.
     * @return The created CameraDTO or error response.
     */
    @PostMapping
    public ResponseEntity<?> createCamera(@RequestBody CameraModel camera) {
        if (camera.getName() == null || camera.getName().trim().isEmpty()) {
            ErrorResponse errorResponse = new ErrorResponse("Camera name is required", "Invalid camera name");
            return new ResponseEntity<>(errorResponse, HttpStatus.BAD_REQUEST);
        }
        String normalizedName = camera.getName().trim();
        var existingCameraOpt = cameraRepository.findByNameIgnoreCase(normalizedName);
        if (existingCameraOpt.isPresent()) {
            CameraModel existingCamera = existingCameraOpt.get();
            String message = String.format(
                "Camera name '%s' already exists (id: %d, device: %s, resolution: %s, fps: %s)",
                existingCamera.getName(),
                existingCamera.getId(),
                existingCamera.getDevice(),
                existingCamera.getResolution(),
                existingCamera.getFps()
            );
            ErrorResponse errorResponse = new ErrorResponse(message, "Duplicate camera name");
            return new ResponseEntity<>(errorResponse, HttpStatus.CONFLICT);
        }
        CameraModel savedCamera = cameraRepository.save(camera);
        return ResponseEntity.status(HttpStatus.CREATED).body(convertToDTO(savedCamera));
    }

    /**
     * Retrieves a camera by its ID.
     *
     * @param id The camera ID.
     * @return The CameraDTO if found.
     */
    @GetMapping("/{id}")
    public ResponseEntity<CameraDTO> getCameraById(@PathVariable Long id) {
        CameraModel camera = cameraRepository.findById(id)
                .orElseThrow(() -> new ResourceNotFoundException("Camera not found with id: " + id));
        return ResponseEntity.ok(convertToDTO(camera));
    }

    /**
     * Updates a camera by its ID.
     *
     * @param id The camera ID.
     * @param cameraDetails The camera details to update.
     * @return The updated CameraDTO.
     */
    @PutMapping("/{id}")
    public CameraDTO updateCamera(@PathVariable Long id, @RequestBody CameraModel cameraDetails) {
        CameraModel camera = cameraRepository.findById(id)
                .orElseThrow(() -> new ResourceNotFoundException("Camera not found with id: " + id));

        // Update only the fields provided in the request body
        if (cameraDetails.getName() != null) camera.setName(cameraDetails.getName());
        if (cameraDetails.getDevice() != null) camera.setDevice(cameraDetails.getDevice());
        if (cameraDetails.getResolution() != null) camera.setResolution(cameraDetails.getResolution());
        if (cameraDetails.getFps() != null) camera.setFps(cameraDetails.getFps());
        if (cameraDetails.getPostUrl() != null) camera.setPostUrl(cameraDetails.getPostUrl());
        if (cameraDetails.getCodec() != null) camera.setCodec(cameraDetails.getCodec());
        if (cameraDetails.getPreset() != null) camera.setPreset(cameraDetails.getPreset());
        if (cameraDetails.getTune() != null) camera.setTune(cameraDetails.getTune());
        if (cameraDetails.getBuffer() != null) camera.setBuffer(cameraDetails.getBuffer());
        if (cameraDetails.getRotation() != null) camera.setRotation(cameraDetails.getRotation());

        CameraModel updatedCamera = cameraRepository.save(camera);
        return convertToDTO(updatedCamera);
    }

    /**
     * Deletes a camera by its ID.
     *
     * @param id The camera ID.
     * @return No content response if deleted.
     */
    @DeleteMapping("/{id}")
    public ResponseEntity<Void> deleteCamera(@PathVariable Long id) {
        CameraModel camera = cameraRepository.findById(id)
                .orElseThrow(() -> new ResourceNotFoundException("Camera not found with id: " + id));

        // Delete the camera
        cameraRepository.delete(camera);

        return ResponseEntity.noContent().build();
    }

    /**
     * Searches cameras by name.
     *
     * @param name The name to search for.
     * @return List of CameraDTO objects matching the name.
     */
    @GetMapping("/search")
    public List<CameraDTO> searchCamerasByName(@RequestParam("name") String name) {
        return cameraRepository.findAll().stream()
                .filter(cam -> cam.getName() != null && cam.getName().toLowerCase().contains(name.toLowerCase()))
                .map(this::convertToDTO)
                .collect(Collectors.toList());
    }

    // Convert CameraModel to CameraDTO
    private CameraDTO convertToDTO(CameraModel camera) {
        CameraDTO cameraDTO = new CameraDTO();
        cameraDTO.setId(camera.getId());
        cameraDTO.setName(camera.getName());
        cameraDTO.setDevice(camera.getDevice());
        cameraDTO.setResolution(camera.getResolution());
        cameraDTO.setFps(camera.getFps());
        cameraDTO.setPostUrl(camera.getPostUrl());
        cameraDTO.setCodec(camera.getCodec());
        cameraDTO.setPreset(camera.getPreset());
        cameraDTO.setTune(camera.getTune());
        cameraDTO.setBuffer(camera.getBuffer());
        cameraDTO.setRotation(camera.getRotation());
        return cameraDTO;
    }

    // Exception handler for ResourceNotFoundException
    @ExceptionHandler(ResourceNotFoundException.class)
    public ResponseEntity<ErrorResponse> handleResourceNotFoundException(ResourceNotFoundException ex) {
        ErrorResponse errorResponse = new ErrorResponse(ex.getMessage(), "Resource not found");
        return new ResponseEntity<>(errorResponse, HttpStatus.NOT_FOUND);
    }
}