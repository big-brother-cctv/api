package com.bigBrother.api.controllers;

import com.bigBrother.api.dtos.UserDTO;
import com.bigBrother.api.exceptions.ErrorResponse;
import com.bigBrother.api.exceptions.ResourceNotFoundException;
import com.bigBrother.api.models.UserModel;
import com.bigBrother.api.repositories.UserRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.security.core.userdetails.UserDetails;
import org.springframework.security.core.annotation.AuthenticationPrincipal;

import java.util.List;
import java.util.stream.Collectors;

/**
 * Controller for managing user resources.
 */
@RestController
@RequestMapping("/api/users")
public class UserController {

    @Autowired
    private UserRepository userRepository;

    /**
     * Retrieves all users.
     *
     * @return List of UserDTO objects.
     */
    @GetMapping
    public List<UserDTO> getAllUsers() {
        return userRepository.findAll().stream().map(this::convertToDTO).collect(Collectors.toList());
    }

    /**
     * Creates a new user.
     *
     * @param user The user model to create.
     * @return The created UserDTO.
     */
    @PostMapping
    public UserDTO createUser(@RequestBody UserModel user) {
        UserModel savedUser = userRepository.save(user);
        return convertToDTO(savedUser);
    }

    /**
     * Retrieves the currently authenticated user's details.
     *
     * @param userDetails The authenticated user's details.
     * @return The UserDTO of the current user.
     */
    @GetMapping("/me")
    public ResponseEntity<UserDTO> getCurrentUser(@AuthenticationPrincipal UserDetails userDetails) {
        String username = userDetails.getUsername();
        UserModel user = userRepository.findByUsername(username)
                .orElseThrow(() -> new ResourceNotFoundException("User not found with username: " + username));
        return ResponseEntity.ok(convertToDTO(user));
    }

    /**
     * Retrieves a user by their ID.
     *
     * @param id The user ID.
     * @return The UserDTO if found.
     */
    @GetMapping("/{id}")
    public ResponseEntity<UserDTO> getUserById(@PathVariable Long id) {
        UserModel user = userRepository.findById(id)
                .orElseThrow(() -> new ResourceNotFoundException("User not found with id: " + id));
        return ResponseEntity.ok(convertToDTO(user));
    }

    /**
     * Updates a user by their ID.
     *
     * @param id The user ID.
     * @param userDetails The user details to update.
     * @return The updated UserDTO.
     */
    @PutMapping("/{id}")
    public UserDTO updateUser(@PathVariable Long id, @RequestBody UserModel userDetails) {
        UserModel user = userRepository.findById(id)
                .orElseThrow(() -> new ResourceNotFoundException("User not found with id: " + id));

        // Update only the fields provided in the request body
        if (userDetails.getName() != null) user.setName(userDetails.getName());
        if (userDetails.getEmail() != null) user.setEmail(userDetails.getEmail());
        if (userDetails.getUsername() != null) user.setUsername(userDetails.getUsername());
        if (userDetails.getPhoto() != null) user.setPhoto(userDetails.getPhoto());

        UserModel updatedUser = userRepository.save(user);
        return convertToDTO(updatedUser);
    }

    /**
     * Deletes a user by their ID.
     *
     * @param id The user ID.
     * @return No content response if deleted.
     */
    @DeleteMapping("/{id}")
    public ResponseEntity<Void> deleteUser(@PathVariable Long id) {
        UserModel user = userRepository.findById(id)
                .orElseThrow(() -> new ResourceNotFoundException("User not found with id: " + id));

        // Delete the user
        userRepository.delete(user);

        return ResponseEntity.noContent().build();
    }

    // Convert UserModel to UserDTO
    private UserDTO convertToDTO(UserModel user) {
        UserDTO userDTO = new UserDTO();
        userDTO.setId(user.getId());
        userDTO.setUsername(user.getUsername());
        userDTO.setEmail(user.getEmail());
        userDTO.setName(user.getName());
        userDTO.setPhoto(user.getPhoto());
        return userDTO;
    }

    // Exception handler for ResourceNotFoundException
    @ExceptionHandler(ResourceNotFoundException.class)
    public ResponseEntity<ErrorResponse> handleResourceNotFoundException(ResourceNotFoundException ex) {
        ErrorResponse errorResponse = new ErrorResponse(ex.getMessage(), "Resource not found");
        return new ResponseEntity<>(errorResponse, HttpStatus.NOT_FOUND);
    }
}