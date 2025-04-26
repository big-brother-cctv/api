package com.bigBrother.api.controllers;

import com.bigBrother.api.models.UserModel;
import com.bigBrother.api.services.JwtService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.authentication.AuthenticationManager;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.authentication.BadCredentialsException;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.web.bind.annotation.*;

import com.bigBrother.api.repositories.UserRepository;

/**
 * Controller for authentication endpoints such as login and registration.
 */
@RestController
@RequestMapping("/api/auth")
public class AuthController {

    @Autowired
    private AuthenticationManager authenticationManager;

    @Autowired
    private JwtService jwtService;

    @Autowired
    private PasswordEncoder passwordEncoder;

    @Autowired
    private UserRepository userRepository;

    /**
     * Authenticates a user and returns a JWT token if credentials are valid.
     *
     * @param authRequest The authentication request containing username and password.
     * @return JWT token if authentication is successful, otherwise an error response.
     */
    @PostMapping("/login")
    public ResponseEntity<?> login(@RequestBody AuthRequest authRequest) {
        try {
            authenticationManager.authenticate(
                new UsernamePasswordAuthenticationToken(authRequest.getUsername(), authRequest.getPassword())
            );
            String token = jwtService.generateToken(authRequest.getUsername());
            return ResponseEntity.ok(new AuthResponse(token));
        } catch (BadCredentialsException e) {
            return ResponseEntity.status(HttpStatus.UNAUTHORIZED).body("Invalid credentials");
        }
    }

    /**
     * Registers a new user if the username and email are unique.
     *
     * @param user The user model containing registration details.
     * @return Success message or error if username/email already exists.
     */
    @PostMapping("/register")
    public ResponseEntity<?> register(@RequestBody UserModel user) {
        System.out.println("Intentando registrar usuario: " + user.getUsername());
        System.out.println("Resultado de findByUsername: " + userRepository.findByUsername(user.getUsername()));
        if (userRepository.findByUsername(user.getUsername()).isPresent()) {
            return ResponseEntity.status(HttpStatus.CONFLICT).body("Username already exists");
        }
    
        if (userRepository.findByEmail(user.getEmail()).isPresent()) {
            return ResponseEntity.status(HttpStatus.CONFLICT).body("Email already exists");
        }
    
        user.setHashedPassword(passwordEncoder.encode(user.getHashedPassword()));
        UserModel savedUser = userRepository.save(user);
        return ResponseEntity.status(HttpStatus.CREATED).body("User registered successfully");
    }
}

/**
 * Request body for authentication containing username and password.
 */
class AuthRequest {
    private String username;
    private String password;

    // Getters y setters
    public String getUsername() {
        return username;
    }

    public void setUsername(String username) {
        this.username = username;
    }

    public String getPassword() {
        return password;
    }

    public void setPassword(String password) {
        this.password = password;
    }
}

/**
 * Response containing the JWT token.
 */
class AuthResponse {
    private String token;

    public AuthResponse(String token) {
        this.token = token;
    }

    public String getToken() {
        return token;
    }
}