package com.bigBrother.api.controllers;

import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.multipart.MultipartFile;
import org.springframework.http.HttpStatus;

@RestController
@RequestMapping("/api/upload")
public class UploadsController {

    @PostMapping("/image")
    public ResponseEntity<String> uploadImage(@RequestParam("file") MultipartFile file) {
        return uploadFile(file, "images");
    }

    @PostMapping("/video")
    public ResponseEntity<String> uploadVideo(@RequestParam("file") MultipartFile file) {
        return uploadFile(file, "videos");
    }

    private ResponseEntity<String> uploadFile(MultipartFile file, String folder) {
        try {
            String contentManagerUrl = "http://content-manager:8181/uploads/" + folder + "/" + file.getOriginalFilename();
            RestTemplate restTemplate = new RestTemplate();
            HttpHeaders headers = new HttpHeaders();
            headers.setContentType(MediaType.parseMediaType(file.getContentType()));
            HttpEntity<byte[]> requestEntity = new HttpEntity<>(file.getBytes(), headers);
    
            restTemplate.exchange(contentManagerUrl, HttpMethod.PUT, requestEntity, String.class);
    
            return ResponseEntity.ok("File uploaded successfully: " + file.getOriginalFilename());
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body("Error uploading file: " + e.getMessage());
        }
    }
}